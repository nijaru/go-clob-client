package clob

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	json "github.com/go-json-experiment/json"
)

// orderBookHashPayload represents the payload used for order book hashing.
// CRITICAL: Field order must match the insertion order in the official SDKs
// because they use SHA1 over JSON-serialized data where keys are in a fixed order.
// DO NOT reorder these fields or the generated hash will not match the server's expected value.
type orderBookHashPayload struct {
	Market         string         `json:"market"`
	AssetID        string         `json:"asset_id"`
	Timestamp      string         `json:"timestamp"`
	Hash           string         `json:"hash"`
	Bids           []OrderSummary `json:"bids"`
	Asks           []OrderSummary `json:"asks"`
	MinOrderSize   string         `json:"min_order_size"`
	TickSize       string         `json:"tick_size"`
	NegRisk        bool           `json:"neg_risk"`
	LastTradePrice string         `json:"last_trade_price"`
}

// GetOrderBookHash returns the server-compatible hash for the supplied orderbook summary.
func (c *Client) GetOrderBookHash(orderbook OrderBookSummary) (string, error) {
	return generateOrderBookHash(orderbook)
}

func generateOrderBookHash(orderbook OrderBookSummary) (string, error) {
	// Implementation parity: empty/nil slices must serialize as [] not null.
	bids := orderbook.Bids
	if bids == nil {
		bids = []OrderSummary{}
	}
	asks := orderbook.Asks
	if asks == nil {
		asks = []OrderSummary{}
	}

	payload := orderBookHashPayload{
		Market:         orderbook.Market,
		AssetID:        orderbook.AssetID,
		Timestamp:      orderbook.Timestamp,
		Hash:           "",
		Bids:           bids,
		Asks:           asks,
		MinOrderSize:   orderbook.MinOrderSize,
		TickSize:       orderbook.TickSize,
		NegRisk:        orderbook.NegRisk,
		LastTradePrice: orderbook.LastTradePrice,
	}

	serialized, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha1.Sum(serialized)
	return hex.EncodeToString(sum[:]), nil
}

// orderBookSummaryHashPayload matches Rust's serialized OrderBookSummaryResponse
// field order and null handling. Unlike GetOrderBookHash, this is the SDK's
// SHA-256 change-detection hash, not the server-compatible SHA-1 hash.
type orderBookSummaryHashPayload struct {
	Market         string         `json:"market"`
	AssetID        string         `json:"asset_id"`
	Timestamp      string         `json:"timestamp"`
	Hash           *string        `json:"hash"`
	Bids           []OrderSummary `json:"bids"`
	Asks           []OrderSummary `json:"asks"`
	MinOrderSize   string         `json:"min_order_size"`
	NegRisk        bool           `json:"neg_risk"`
	TickSize       string         `json:"tick_size"`
	LastTradePrice *string        `json:"last_trade_price"`
}

// GetOrderBookSHA256Hash returns Rust's OrderBookSummaryResponse hash. It
// serializes the typed response shape and hashes it with SHA-256.
func (c *Client) GetOrderBookSHA256Hash(orderbook OrderBookSummary) (string, error) {
	return generateOrderBookSHA256Hash(orderbook)
}

// GetOrderBookSummaryHash is an explicit alias for GetOrderBookSHA256Hash.
func (c *Client) GetOrderBookSummaryHash(orderbook OrderBookSummary) (string, error) {
	return generateOrderBookSHA256Hash(orderbook)
}

func generateOrderBookSHA256Hash(orderbook OrderBookSummary) (string, error) {
	market, err := normalizeOrderBookHashBytes32(orderbook.Market)
	if err != nil {
		return "", fmt.Errorf("orderbook market: %w", err)
	}
	assetID, err := normalizeOrderBookHashAssetID(orderbook.AssetID)
	if err != nil {
		return "", fmt.Errorf("orderbook asset ID: %w", err)
	}

	bids := orderbook.Bids
	if bids == nil {
		bids = []OrderSummary{}
	}
	asks := orderbook.Asks
	if asks == nil {
		asks = []OrderSummary{}
	}

	var hash *string
	if orderbook.Hash != "" {
		value := orderbook.Hash
		hash = &value
	}
	var lastTradePrice *string
	if orderbook.LastTradePrice != "" {
		value := orderbook.LastTradePrice
		lastTradePrice = &value
	}

	payload := orderBookSummaryHashPayload{
		Market:         market,
		AssetID:        assetID,
		Timestamp:      orderbook.Timestamp,
		Hash:           hash,
		Bids:           bids,
		Asks:           asks,
		MinOrderSize:   orderbook.MinOrderSize,
		NegRisk:        orderbook.NegRisk,
		TickSize:       string(orderbook.TickSize),
		LastTradePrice: lastTradePrice,
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal orderbook: %w", err)
	}

	sum := sha256.Sum256(serialized)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeOrderBookHashBytes32(value string) (string, error) {
	value = strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	if value == "" {
		return "", fmt.Errorf("value is empty")
	}
	number, ok := new(big.Int).SetString(value, 16)
	if !ok || number.Sign() < 0 || number.BitLen() > 256 {
		return "", fmt.Errorf("%q is not a bytes32 hex value", value)
	}
	return fmt.Sprintf("0x%064x", number), nil
}

func normalizeOrderBookHashAssetID(value string) (string, error) {
	base := 10
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		value = value[2:]
	}
	if value == "" {
		return "", fmt.Errorf("value is empty")
	}
	number, ok := new(big.Int).SetString(value, base)
	if !ok || number.Sign() < 0 || number.BitLen() > 256 {
		return "", fmt.Errorf("%q is not a uint256 value", value)
	}
	return fmt.Sprintf("0x%064x", number), nil
}
