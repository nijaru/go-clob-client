package clob

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

type comboInbound struct {
	Type               string                       `json:"type"`
	Success            *bool                        `json:"success"`
	Error              string                       `json:"error"`
	Code               string                       `json:"code"`
	ErrorID            string                       `json:"error_id"`
	RequestType        string                       `json:"request_type"`
	RFQID              string                       `json:"rfq_id"`
	QuoteID            string                       `json:"quote_id"`
	RequestorPublicID  string                       `json:"requestor_public_id"`
	RequesterID        string                       `json:"requester_id"`
	LegPositionIDs     []string                     `json:"leg_position_ids"`
	ConditionID        string                       `json:"condition_id"`
	YesPositionID      string                       `json:"yes_position_id"`
	NoPositionID       string                       `json:"no_position_id"`
	Direction          RFQDirection                 `json:"direction"`
	Side               RFQSide                      `json:"side"`
	RequestedSize      *builderRfqRequestedSize     `json:"requested_size"`
	SubmissionDeadline *int64                       `json:"submission_deadline"`
	Signer             string                       `json:"signer_address"`
	Maker              string                       `json:"maker_address"`
	SignatureType      *SignatureType               `json:"signature_type"`
	FillSizeE6         string                       `json:"fill_size_e6"`
	PriceE6            string                       `json:"price_e6"`
	SizeE6             string                       `json:"size_e6"`
	ConfirmBy          *int64                       `json:"confirm_by"`
	Decision           ComboRFQConfirmationDecision `json:"decision"`
	Status             ComboRFQStatus               `json:"status"`
	TxHash             *string                      `json:"tx_hash"`
	ExecutedAt         *int64                       `json:"executed_at"`
}

func comboConditionID(value string) (string, bool) {
	value = strings.ToLower(value)
	if !strings.HasPrefix(value, "0x03") {
		return "", false
	}
	if len(value) == 66 && (strings.HasSuffix(value, "00") || strings.HasSuffix(value, "01")) {
		value = value[:64]
	}
	if len(value) != 64 {
		return "", false
	}
	_, err := hex.DecodeString(value[2:])
	return value, err == nil
}

func (m comboInbound) validMarket() bool {
	if m.RFQID == "" || m.Side != RFQSideYes ||
		(m.Direction != RFQDirectionBuy && m.Direction != RFQDirectionSell) ||
		m.LegPositionIDs == nil {
		return false
	}
	for _, id := range m.LegPositionIDs {
		if !comboPositionValid(id) {
			return false
		}
	}
	_, ok := comboConditionID(m.ConditionID)
	return ok
}

func (m comboInbound) validPositions() bool {
	return comboPositionValid(m.YesPositionID) && comboPositionValid(m.NoPositionID)
}

func (m comboInbound) reference() ComboRFQQuoteReference {
	return ComboRFQQuoteReference{RFQID: m.RFQID, QuoteID: m.QuoteID}
}

func (s *ComboRFQSession) readLoop() {
	defer close(s.readerDone)
	defer close(s.events)
	for {
		_, data, err := s.conn.Read(s.ctx)
		if err != nil {
			s.finish(fmt.Errorf("combo RFQ connection lost: %w", err))
			return
		}
		// Stable SDKs drop unknown and malformed frames, including unreadable acks.
		// A command waiting for such an ack still has a bounded timeout.
		var m comboInbound
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		event := s.handleInbound(m)
		if event == nil {
			continue
		}
		select {
		case <-s.done:
			return
		case s.events <- event:
		default:
			s.finish(fmt.Errorf("combo RFQ event buffer exhausted"))
			return
		}
	}
}

func (s *ComboRFQSession) handleInbound(m comboInbound) ComboRFQEvent {
	switch m.Type {
	case "auth":
		if m.Success == nil {
			return nil
		}
		var err error
		if !*m.Success {
			err = fmt.Errorf("combo RFQ authentication failed: %s", m.Error)
		}
		s.resolve(comboAckKey{kind: "auth"}, comboAckResult{err: err})
	case "ACK_RFQ_QUOTE", "ACK_RFQ_QUOTE_CANCEL", "ACK_RFQ_CONFIRMATION_RESPONSE":
		if m.RFQID == "" || m.QuoteID == "" {
			return nil
		}
		quoteID := m.QuoteID
		if m.Type == "ACK_RFQ_QUOTE" {
			quoteID = ""
		}
		if m.Type == "ACK_RFQ_CONFIRMATION_RESPONSE" && m.Decision != ComboRFQConfirm &&
			m.Decision != ComboRFQDecline {
			return nil
		}
		s.resolve(
			comboAckKey{m.Type, m.RFQID, quoteID},
			comboAckResult{reference: m.reference(), decision: m.Decision},
		)
	case "RFQ_ERROR":
		if m.Code == "" || m.Error == "" {
			return nil
		}
		kind := ""
		switch m.RequestType {
		case "RFQ_QUOTE":
			kind = "ACK_RFQ_QUOTE"
		case "RFQ_QUOTE_CANCEL":
			kind = "ACK_RFQ_QUOTE_CANCEL"
		case "RFQ_CONFIRMATION_RESPONSE":
			kind = "ACK_RFQ_CONFIRMATION_RESPONSE"
		default:
			return nil
		}
		if m.RFQID == "" || (m.RequestType != "RFQ_QUOTE" && m.QuoteID == "") {
			s.finish(fmt.Errorf("uncorrelated combo RFQ error"))
			return nil
		}
		quoteID := m.QuoteID
		if m.RequestType == "RFQ_QUOTE" {
			quoteID = ""
		}
		s.resolve(
			comboAckKey{kind, m.RFQID, quoteID},
			comboAckResult{
				err: &ComboRFQCommandError{
					RequestType: m.RequestType,
					RFQID:       m.RFQID,
					QuoteID:     m.QuoteID,
					Code:        m.Code,
					ErrorID:     m.ErrorID,
					Message:     m.Error,
				},
			},
		)
	case "RFQ_REQUEST":
		if !m.validMarket() || !m.validPositions() || m.RequestorPublicID == "" ||
			m.RequestedSize == nil ||
			m.SubmissionDeadline == nil {
			return nil
		}
		if m.RequestedSize.Unit != RFQSizeUnitShares &&
			m.RequestedSize.Unit != RFQSizeUnitNotional {
			return nil
		}
		value, err := e6ToDecimal(m.RequestedSize.ValueE6)
		if err != nil {
			return nil
		}
		condition, _ := comboConditionID(m.ConditionID)
		return ComboRFQQuoteRequest{
			RFQID:              m.RFQID,
			RequestorPublicID:  m.RequestorPublicID,
			LegPositionIDs:     m.LegPositionIDs,
			ConditionID:        condition,
			YesPositionID:      m.YesPositionID,
			NoPositionID:       m.NoPositionID,
			Direction:          m.Direction,
			Side:               m.Side,
			RequestedSize:      ComboRFQRequestedSize{Unit: m.RequestedSize.Unit, Value: value},
			SubmissionDeadline: *m.SubmissionDeadline,
		}
	case "RFQ_CONFIRMATION_REQUEST":
		if !m.validMarket() || !m.validPositions() || m.QuoteID == "" ||
			!common.IsHexAddress(m.Signer) ||
			!common.IsHexAddress(m.Maker) ||
			m.SignatureType == nil ||
			*m.SignatureType > SignatureTypePoly1271 ||
			*m.SignatureType < SignatureTypeEOA ||
			m.ConfirmBy == nil {
			return nil
		}
		size, err := e6ToDecimal(m.FillSizeE6)
		if err != nil {
			return nil
		}
		price, err := e6ToDecimal(m.PriceE6)
		if err != nil {
			return nil
		}
		condition, _ := comboConditionID(m.ConditionID)
		return ComboRFQConfirmationRequest{
			ComboRFQQuoteReference: m.reference(),
			SignerAddress:          m.Signer,
			MakerAddress:           m.Maker,
			SignatureType:          *m.SignatureType,
			LegPositionIDs:         m.LegPositionIDs,
			ConditionID:            condition,
			YesPositionID:          m.YesPositionID,
			NoPositionID:           m.NoPositionID,
			Direction:              m.Direction,
			Side:                   m.Side,
			FillSize:               size,
			Price:                  price,
			ConfirmBy:              *m.ConfirmBy,
		}
	case "RFQ_EXECUTION_UPDATE":
		if m.RFQID == "" {
			return nil
		}
		switch m.Status {
		case ComboRFQMatched, ComboRFQMined, ComboRFQConfirmed, ComboRFQRetrying, ComboRFQFailed:
		default:
			return nil
		}
		tx := ""
		if m.TxHash != nil {
			tx = *m.TxHash
			if !comboTransactionHashValid(tx) {
				return nil
			}
		}
		return ComboRFQExecutionUpdate{RFQID: m.RFQID, Status: m.Status, TxHash: tx}
	case "RFQ_TRADE":
		if !m.validMarket() || m.RequesterID == "" || m.ExecutedAt == nil {
			return nil
		}
		price, err := e6ToDecimal(m.PriceE6)
		if err != nil {
			return nil
		}
		size, err := e6ToDecimal(m.SizeE6)
		if err != nil {
			return nil
		}
		condition, _ := comboConditionID(m.ConditionID)
		return ComboRFQTrade{
			RFQID:          m.RFQID,
			RequesterID:    m.RequesterID,
			ConditionID:    condition,
			LegPositionIDs: m.LegPositionIDs,
			Direction:      m.Direction,
			Side:           m.Side,
			Price:          price,
			Size:           size,
			ExecutedAt:     *m.ExecutedAt,
		}
	}
	return nil
}
