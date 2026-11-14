package clob

import (
	"context"
	"encoding/hex"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

var (
	ErrWalletOrdersDepositWalletRequired = errors.New("wallet-wide orders require a Deposit Wallet")
	ErrWalletOrdersOwnerRequired         = errors.New("wallet-wide cancellation requires the Deposit Wallet owner")
)

// WalletOpenOrder represents an open order placed by the Deposit Wallet owner
// or a non-revoked session signer. Owner is nil when the API-key ID is withheld.
// Financial amounts are exact decimal strings, never float64.
type WalletOpenOrder struct {
	ID            string     `json:"id"`
	AssetID       string     `json:"asset_id"`
	Market        string     `json:"market"`
	Owner         *string    `json:"owner"`
	MakerAddress  string     `json:"maker_address"`
	SignerAddress string     `json:"signer_address"`
	Side          string     `json:"side"`
	Price         string     `json:"price"`
	OriginalSize  string     `json:"original_size"`
	SizeMatched   string     `json:"size_matched"`
	Outcome       string     `json:"outcome"`
	OrderType     string     `json:"order_type"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"-"`
	ExpiresAt     *time.Time `json:"-"`
}

// UnmarshalJSON preserves numeric lexemes and normalizes the exchange's
// epoch-second, epoch-millisecond and ISO timestamp representations.
func (o *WalletOpenOrder) UnmarshalJSON(data []byte) error {
	fields, err := responseFields(data)
	if err != nil {
		return fmt.Errorf("wallet order: %w", err)
	}
	if err := normalizeResponseAsset(fields, "asset_id", "token_id"); err != nil {
		return fmt.Errorf("wallet order: %w", err)
	}
	if err := normalizeResponseStrings(fields, "price", "original_size", "size_matched"); err != nil {
		return fmt.Errorf("wallet order: %w", err)
	}

	created, _, err := normalizeOrderTimestamp(fields["created_at"], false, false)
	if err != nil {
		return fmt.Errorf("wallet order created_at: %w", err)
	}
	if created == nil {
		return errors.New("wallet order missing created_at")
	}
	expires, _, err := normalizeOrderTimestamp(fields["expiration"], true, true)
	if err != nil {
		return fmt.Errorf("wallet order expiration: %w", err)
	}
	delete(fields, "created_at")
	delete(fields, "expiration")

	type wire WalletOpenOrder
	var decoded wire
	if err := decodeResponseFields(fields, &decoded); err != nil {
		return fmt.Errorf("wallet order: %w", err)
	}
	if decoded.ID == "" || decoded.AssetID == "" || decoded.Market == "" ||
		decoded.Price == "" || decoded.OriginalSize == "" || decoded.SizeMatched == "" ||
		!common.IsHexAddress(decoded.MakerAddress) ||
		!common.IsHexAddress(decoded.SignerAddress) {
		return errors.New("wallet order missing or invalid required fields")
	}
	if decoded.Owner != nil && *decoded.Owner == "" {
		decoded.Owner = nil
	}
	decoded.CreatedAt = *created
	decoded.ExpiresAt = expires
	*o = WalletOpenOrder(decoded)
	return nil
}

// WalletOpenOrdersPage uses an opaque cursor, with an empty next_cursor
// indicating completion. Empty pages may still have a continuation.
type WalletOpenOrdersPage struct {
	Data       []WalletOpenOrder `json:"data"`
	NextCursor string            `json:"next_cursor"`
}

func (p *WalletOpenOrdersPage) UnmarshalJSON(data []byte) error {
	var wire struct {
		Data       *[]WalletOpenOrder `json:"data"`
		NextCursor *string            `json:"next_cursor"`
	}
	if err := stdjson.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("wallet orders page: %w", err)
	}
	if wire.Data == nil || wire.NextCursor == nil {
		return errors.New("wallet orders page missing data or next_cursor")
	}
	p.Data = *wire.Data
	p.NextCursor = *wire.NextCursor
	return nil
}

type WalletOpenOrdersParams struct {
	Market          string
	AssetID         string
	SignerAddresses []string // nil means all signers; one to 50 when supplied
	Cursor          string
}

func (p WalletOpenOrdersParams) query() (url.Values, error) {
	q := make(url.Values, 4)
	if p.Market != "" {
		q.Set("market", p.Market)
	}
	if p.AssetID != "" {
		q.Set("asset_id", p.AssetID)
	}
	if p.SignerAddresses != nil {
		if len(p.SignerAddresses) == 0 || len(p.SignerAddresses) > 50 {
			return nil, fmt.Errorf("signer addresses must contain 1 to 50 entries")
		}
		for _, address := range p.SignerAddresses {
			if !common.IsHexAddress(address) {
				return nil, fmt.Errorf("invalid signer address %q", address)
			}
		}
		q.Set("session_keys", strings.Join(p.SignerAddresses, ","))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	return q, nil
}

func (c *AuthenticatedClient) requireDepositWalletOrders() error {
	if c.signatureType != SignatureTypePoly1271 {
		return ErrWalletOrdersDepositWalletRequired
	}
	return nil
}

func (c *AuthenticatedClient) requireWalletOrderOwner() error {
	if err := c.requireDepositWalletOrders(); err != nil {
		return err
	}
	owner, err := c.isDepositWalletOwner()
	if err != nil {
		return err
	}
	if !owner {
		return ErrWalletOrdersOwnerRequired
	}
	return nil
}

// GetWalletOpenOrdersPage lists open orders across Deposit Wallet signers,
// including expired (but not revoked) session signers. The server controls
// page size; a page can be empty even when a continuation exists.
func (c *AuthenticatedClient) GetWalletOpenOrdersPage(
	ctx context.Context,
	params WalletOpenOrdersParams,
) (*WalletOpenOrdersPage, error) {
	if err := c.requireDepositWalletOrders(); err != nil {
		return nil, err
	}
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	var out WalletOpenOrdersPage
	if err := c.getJSON(
		ctx, walletOpenOrdersEndpoint, query, polyhttp.AuthL2Builder, &out,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

// IterWalletOpenOrders traverses opaque cursors without treating an empty page
// as completion. Cursor cycles return ErrPaginationStalled rather than truncating.
func (c *AuthenticatedClient) IterWalletOpenOrders(
	ctx context.Context,
	params WalletOpenOrdersParams,
) iter.Seq2[WalletOpenOrder, error] {
	return func(yield func(WalletOpenOrder, error) bool) {
		cursor := params.Cursor
		seen := make(map[string]struct{})
		for {
			if _, exists := seen[cursor]; exists {
				var zero WalletOpenOrder
				yield(zero, fmt.Errorf("%w: cursor cycle %q", ErrPaginationStalled, cursor))
				return
			}
			seen[cursor] = struct{}{}
			params.Cursor = cursor
			page, err := c.GetWalletOpenOrdersPage(ctx, params)
			if err != nil {
				var zero WalletOpenOrder
				yield(zero, err)
				return
			}
			for _, order := range page.Data {
				if !yield(order, nil) {
					return
				}
			}
			if page.NextCursor == "" {
				return
			}
			cursor = page.NextCursor
		}
	}
}

// CancelWalletOrders cancels 1-1000 order hashes across a Deposit Wallet,
// regardless of which session signer placed them. Only the wallet owner may
// cancel. Responses may report partial failures. Never retry an uncertain
// cancellation automatically.
func (c *AuthenticatedClient) CancelWalletOrders(
	ctx context.Context,
	orderIDs []string,
) (*CancelOrdersResponse, error) {
	if err := c.requireWalletOrderOwner(); err != nil {
		return nil, err
	}
	if err := validateWalletCancelIDs(orderIDs); err != nil {
		return nil, err
	}
	var out CancelOrdersResponse
	request := struct {
		OrderIDs []string `json:"order_ids"`
	}{OrderIDs: orderIDs}
	err := c.cancelTradingJSON(
		ctx, cancelWalletOrdersEndpoint, request, polyhttp.AuthL2Builder, &out,
	)
	return &out, err
}

// CancelAllWalletOrders cancels all open orders across the Deposit Wallet,
// rather than only those placed with the current API key.
func (c *AuthenticatedClient) CancelAllWalletOrders(
	ctx context.Context,
) (*CancelOrdersResponse, error) {
	if err := c.requireWalletOrderOwner(); err != nil {
		return nil, err
	}
	var out CancelOrdersResponse
	err := c.cancelTradingJSON(
		ctx, cancelWalletOrdersEndpoint,
		struct {
			All bool `json:"all"`
		}{All: true},
		polyhttp.AuthL2Builder, &out,
	)
	return &out, err
}

func validateWalletCancelIDs(orderIDs []string) error {
	if len(orderIDs) == 0 || len(orderIDs) > 1000 {
		return fmt.Errorf("wallet cancellation requires 1 to 1000 order hashes")
	}
	for _, id := range orderIDs {
		if len(id) != 66 || !strings.HasPrefix(id, "0x") {
			return fmt.Errorf("invalid wallet order hash %q", id)
		}
		if _, err := hex.DecodeString(id[2:]); err != nil {
			return fmt.Errorf("invalid wallet order hash %q: %w", id, err)
		}
	}
	return nil
}
