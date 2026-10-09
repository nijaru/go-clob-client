package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
)

var (
	ErrComboRFQSessionClosed  = errors.New("combo RFQ session closed")
	ErrComboRFQCommandPending = errors.New(
		"combo RFQ command with the same acknowledgement key is already pending",
	)
)

type (
	comboAckKey    struct{ kind, rfqID, quoteID string }
	comboAckResult struct {
		reference ComboRFQQuoteReference
		decision  ComboRFQConfirmationDecision
		err       error
	}
)

// ComboRFQSession owns a single authenticated socket. It never reconnects or
// replays trading commands automatically. After Done closes, open a new session
// explicitly. Independent sessions have no shared lifecycle or pending state.
// Methods are safe for concurrent use; Events is a single-consumer stream.
type ComboRFQSession struct {
	client     *AuthenticatedClient
	conn       *websocket.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	ackTimeout time.Duration
	events     chan ComboRFQEvent
	done       chan struct{}
	readerDone chan struct{}
	writeGate  chan struct{}
	mu         sync.Mutex
	closed     bool
	err        error
	pending    map[comboAckKey]chan comboAckResult
}

// OpenComboRFQSession authenticates with an immutable credential snapshot and
// the account's order identity. Builder credentials are not needed for quoting.
// The supplied context owns the entire session, including commands and reads.
func (c *AuthenticatedClient) OpenComboRFQSession(
	ctx context.Context,
	options ComboRFQSessionOptions,
) (*ComboRFQSession, error) {
	if err := c.requireComboAccount(); err != nil {
		return nil, err
	}
	if options.AuthTimeout < 0 || options.AckTimeout < 0 {
		return nil, fmt.Errorf("combo RFQ timeouts must not be negative")
	}
	if options.URL == "" {
		options.URL = DefaultComboRFQQuoterURL
	}
	if options.AuthTimeout == 0 {
		options.AuthTimeout = 30 * time.Second
	}
	if options.AckTimeout == 0 {
		options.AckTimeout = 30 * time.Second
	}
	auth, err := c.DeriveWSAuth(ctx)
	if err != nil {
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	dialCtx, dialCancel := context.WithTimeout(sessionCtx, options.AuthTimeout)
	conn, response, err := websocket.Dial(
		dialCtx,
		options.URL,
		&websocket.DialOptions{HTTPHeader: options.Header.Clone(), HTTPClient: options.HTTPClient},
	)
	dialCancel()
	if err != nil {
		cancel()
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, fmt.Errorf("combo RFQ dial: %w", err)
	}
	conn.SetReadLimit(1 << 20)
	s := &ComboRFQSession{
		client:     c,
		conn:       conn,
		ctx:        sessionCtx,
		cancel:     cancel,
		ackTimeout: options.AckTimeout,
		events:     make(chan ComboRFQEvent, 1024),
		done:       make(chan struct{}),
		readerDone: make(chan struct{}),
		writeGate:  make(chan struct{}, 1),
		pending:    make(map[comboAckKey]chan comboAckResult),
	}
	go s.readLoop()
	go func() {
		select {
		case <-ctx.Done():
			s.finish(ctx.Err())
		case <-s.done:
		}
	}()
	signer := c.Address()
	if c.signatureType == SignatureTypePoly1271 {
		signer = c.comboMakerAddress()
	}
	message := struct {
		Type string `json:"type"`
		Auth struct {
			APIKey     string `json:"apiKey"`
			Passphrase string `json:"passphrase"`
			Secret     string `json:"secret"`
		} `json:"auth"`
		Identity struct {
			Signer        string        `json:"signer_address"`
			Maker         string        `json:"maker_address"`
			SignatureType SignatureType `json:"signature_type"`
		} `json:"identity"`
	}{Type: "auth"}
	message.Auth.APIKey, message.Auth.Passphrase, message.Auth.Secret = auth.Key, auth.Passphrase, auth.Secret
	message.Identity.Signer, message.Identity.Maker, message.Identity.SignatureType = signer, c.comboMakerAddress(), c.signatureType
	authCtx, authCancel := context.WithTimeout(ctx, options.AuthTimeout)
	defer authCancel()
	if _, err := s.command(authCtx, comboAckKey{kind: "auth"}, message); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *ComboRFQSession) Events() <-chan ComboRFQEvent { return s.events }
func (s *ComboRFQSession) Done() <-chan struct{}        { return s.done }

// Err reports the terminal transport/protocol/context error; normal Close is nil.
func (s *ComboRFQSession) Err() error   { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *ComboRFQSession) Close() error { s.finish(nil); <-s.readerDone; return nil }

func (s *ComboRFQSession) finish(err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed, s.err = true, err
	pendingErr := err
	if pendingErr == nil {
		pendingErr = ErrComboRFQSessionClosed
	}
	for key, ch := range s.pending {
		ch <- comboAckResult{err: pendingErr}
		delete(s.pending, key)
	}
	close(s.done)
	s.mu.Unlock()
	s.cancel()
	_ = s.conn.CloseNow()
}

func (s *ComboRFQSession) resolve(key comboAckKey, result comboAckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch := s.pending[key]; ch != nil {
		delete(s.pending, key)
		ch <- result
	}
}

// Acknowledgements carry no client command ID. Reject overlapping same-key
// commands, and terminate after an ambiguous timeout/cancellation so a late ack
// cannot accidentally settle a later command on this socket.
func (s *ComboRFQSession) command(
	ctx context.Context,
	key comboAckKey,
	message any,
) (comboAckResult, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return comboAckResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.ackTimeout)
	defer cancel()
	select {
	case s.writeGate <- struct{}{}:
	case <-ctx.Done():
		return comboAckResult{}, ctx.Err()
	case <-s.done:
		return comboAckResult{}, ErrComboRFQSessionClosed
	}
	if err := ctx.Err(); err != nil {
		<-s.writeGate
		return comboAckResult{}, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		<-s.writeGate
		return comboAckResult{}, ErrComboRFQSessionClosed
	}
	if s.pending[key] != nil {
		s.mu.Unlock()
		<-s.writeGate
		return comboAckResult{}, ErrComboRFQCommandPending
	}
	ch := make(chan comboAckResult, 1)
	s.pending[key] = ch
	s.mu.Unlock()
	writeCtx, writeCancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, writeCancel)
	err = s.conn.Write(writeCtx, websocket.MessageText, payload)
	stop()
	writeCancel()
	<-s.writeGate
	if err != nil {
		s.finish(err)
		return comboAckResult{}, err
	}
	select {
	case result := <-ch:
		return result, result.err
	case <-ctx.Done():
		s.finish(ctx.Err())
		return comboAckResult{}, ctx.Err()
	}
}

// Quote signs and submits the exact maker order; the result is a gateway ack,
// not a fill. Price/Size are human-readable decimals, never float64.
func (s *ComboRFQSession) Quote(
	ctx context.Context,
	request ComboRFQQuoteRequest,
	response ComboRFQQuoteResponse,
) (ComboRFQQuoteReference, error) {
	if err := ctx.Err(); err != nil {
		return ComboRFQQuoteReference{}, err
	}
	quote, err := s.client.buildComboQuoterQuote(request, response)
	if err != nil {
		return ComboRFQQuoteReference{}, err
	}
	result, err := s.command(ctx, comboAckKey{kind: "ACK_RFQ_QUOTE", rfqID: request.RFQID}, quote)
	return result.reference, err
}

// CancelQuote acknowledges processing, not guaranteed withdrawal of a quote
// that may already have been selected.
func (s *ComboRFQSession) CancelQuote(
	ctx context.Context,
	reference ComboRFQQuoteReference,
) (ComboRFQQuoteReference, error) {
	if reference.RFQID == "" || reference.QuoteID == "" {
		return ComboRFQQuoteReference{}, fmt.Errorf("combo cancellation requires RFQID and QuoteID")
	}
	signer := s.client.Address()
	if s.client.signatureType == SignatureTypePoly1271 {
		signer = s.client.comboMakerAddress()
	}
	message := struct {
		Type    string `json:"type"`
		RFQID   string `json:"rfq_id"`
		QuoteID string `json:"quote_id"`
		Signer  string `json:"signer_address"`
		Maker   string `json:"maker_address"`
	}{"RFQ_QUOTE_CANCEL", reference.RFQID, reference.QuoteID, signer, s.client.comboMakerAddress()}
	result, err := s.command(
		ctx,
		comboAckKey{"ACK_RFQ_QUOTE_CANCEL", reference.RFQID, reference.QuoteID},
		message,
	)
	return result.reference, err
}

// RespondToConfirmation explicitly confirms or declines the maker's last look.
func (s *ComboRFQSession) RespondToConfirmation(
	ctx context.Context,
	reference ComboRFQQuoteReference,
	decision ComboRFQConfirmationDecision,
) (ComboRFQConfirmationAck, error) {
	if reference.RFQID == "" || reference.QuoteID == "" ||
		(decision != ComboRFQConfirm && decision != ComboRFQDecline) {
		return ComboRFQConfirmationAck{}, fmt.Errorf(
			"invalid combo confirmation reference or decision",
		)
	}
	message := struct {
		Type     string                       `json:"type"`
		RFQID    string                       `json:"rfq_id"`
		QuoteID  string                       `json:"quote_id"`
		Decision ComboRFQConfirmationDecision `json:"decision"`
	}{"RFQ_CONFIRMATION_RESPONSE", reference.RFQID, reference.QuoteID, decision}
	result, err := s.command(
		ctx,
		comboAckKey{"ACK_RFQ_CONFIRMATION_RESPONSE", reference.RFQID, reference.QuoteID},
		message,
	)
	if err == nil && result.decision != decision {
		err = fmt.Errorf("combo RFQ confirmation acknowledgement decision mismatch")
		s.finish(err)
	}
	return ComboRFQConfirmationAck{
		ComboRFQQuoteReference: result.reference,
		Decision:               result.decision,
	}, err
}
