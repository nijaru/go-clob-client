package clob

import "encoding/json"

type Credentials struct {
	Key        string `json:"key"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

type apiKeyRaw struct {
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

type APIKeysResponse struct {
	APIKeys []Credentials `json:"apiKeys"`
}

type BanStatus struct {
	ClosedOnly bool `json:"closed_only"`
}

type BookParams struct {
	TokenID string `json:"token_id"`
}

type OrderSummary struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type OrderBookSummary struct {
	Market         string         `json:"market"`
	AssetID        string         `json:"asset_id"`
	Timestamp      string         `json:"timestamp"`
	Bids           []OrderSummary `json:"bids"`
	Asks           []OrderSummary `json:"asks"`
	MinOrderSize   string         `json:"min_order_size"`
	TickSize       string         `json:"tick_size"`
	NegRisk        bool           `json:"neg_risk"`
	LastTradePrice string         `json:"last_trade_price"`
	Hash           string         `json:"hash"`
}

type TickSizeResponse struct {
	MinimumTickSize string `json:"minimum_tick_size"`
}

type NegRiskResponse struct {
	NegRisk bool `json:"neg_risk"`
}

type FeeRateResponse struct {
	FeeRateBps int64 `json:"fee_rate_bps"`
}

type PriceResponse struct {
	Price   string `json:"price"`
	Side    string `json:"side,omitempty"`
	TokenID string `json:"token_id,omitempty"`
}

type SpreadResponse struct {
	Spread string `json:"spread"`
}

type CursorPage struct {
	Limit      int               `json:"limit"`
	Count      int               `json:"count"`
	NextCursor string            `json:"next_cursor"`
	Data       []json.RawMessage `json:"data"`
}

type OrderPayload struct {
	OrderID string `json:"orderID"`
}

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type OrderType string

const (
	OrderTypeGTC OrderType = "GTC"
	OrderTypeFOK OrderType = "FOK"
	OrderTypeGTD OrderType = "GTD"
	OrderTypeFAK OrderType = "FAK"
)

type SignedOrder struct {
	Salt          string        `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	Taker         string        `json:"taker"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Expiration    string        `json:"expiration"`
	Nonce         string        `json:"nonce"`
	FeeRateBps    string        `json:"feeRateBps"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Signature     string        `json:"signature"`
}

type PostOrderRequest struct {
	Order     SignedOrder `json:"order"`
	Owner     string      `json:"owner"`
	OrderType OrderType   `json:"orderType"`
	DeferExec bool        `json:"deferExec"`
	PostOnly  bool        `json:"postOnly,omitempty"`
}

type OpenOrderParams struct {
	ID      string
	Market  string
	AssetID string
}

type TradeParams struct {
	ID           string
	MakerAddress string
	Market       string
	AssetID      string
	Before       string
	After        string
}
