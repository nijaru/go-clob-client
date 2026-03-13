package clob

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
)

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
	payload := orderBookHashPayload{
		Market:         orderbook.Market,
		AssetID:        orderbook.AssetID,
		Timestamp:      orderbook.Timestamp,
		Hash:           "",
		Bids:           append([]OrderSummary{}, orderbook.Bids...),
		Asks:           append([]OrderSummary{}, orderbook.Asks...),
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
