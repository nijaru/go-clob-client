package clob

import (
	"crypto/sha1"
	"encoding/hex"
	json "encoding/json/v2"
)

// orderBookHashPayload represents the payload used for order book hashing.
// CRITICAL: Field order must match the insertion order in the official TypeScript/Python SDKs
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
	// Python parity: empty/nil slices must serialize as [] not null.
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
