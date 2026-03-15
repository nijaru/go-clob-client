package clob

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// OrderPayload identifies a single order in cancel and lookup requests.
type OrderPayload struct {
	OrderID string `json:"orderID"`
}

// PostOrderResponse is the response payload returned after posting an order.
type PostOrderResponse struct {
	Success            bool     `json:"success"`
	ErrorMsg           string   `json:"errorMsg"`
	OrderID            string   `json:"orderID"`
	TransactionsHashes []string `json:"transactionsHashes"`
	Status             string   `json:"status"`
	TakingAmount       string   `json:"takingAmount"`
	MakingAmount       string   `json:"makingAmount"`
	TradeIDs           []string `json:"trade_ids"`
}

// CancelOrdersResponse reports which orders were canceled successfully.
type CancelOrdersResponse struct {
	Canceled    []string
	NotCanceled map[string]string
}

// Side is the taker or maker side for an order or trade.
type Side string

const (
	// SideBuy is the buy side.
	SideBuy Side = "BUY"
	// SideSell is the sell side.
	SideSell Side = "SELL"
)

// OrderType controls how the exchange should handle the order.
type OrderType string

const (
	// OrderTypeGTC keeps an order on the book until it is filled or canceled.
	OrderTypeGTC OrderType = "GTC"
	// OrderTypeFOK requires the entire order to fill immediately or fail.
	OrderTypeFOK OrderType = "FOK"
	// OrderTypeGTD keeps an order active until its expiration.
	OrderTypeGTD OrderType = "GTD"
	// OrderTypeFAK fills whatever can trade immediately and cancels the rest.
	OrderTypeFAK OrderType = "FAK"
)

// TickSize identifies the minimum supported market tick size.
type TickSize string

const (
	// TickSizeTenth rounds prices to one decimal place.
	TickSizeTenth TickSize = "0.1"
	// TickSizeHundredth rounds prices to two decimal places.
	TickSizeHundredth TickSize = "0.01"
	// TickSizeThousandth rounds prices to three decimal places.
	TickSizeThousandth TickSize = "0.001"
	// TickSizeTenThousand rounds prices to four decimal places.
	TickSizeTenThousand TickSize = "0.0001"
)

type roundConfig struct {
	Price  uint8
	Size   uint8
	Amount uint8
}

// CreateOrderOptions overrides market-derived trading defaults.
type CreateOrderOptions struct {
	TickSize TickSize
	NegRisk  *bool
}

// OpenOrder is an authenticated open-order record.
type OpenOrder struct {
	ID              string   `json:"id"`
	Status          string   `json:"status"`
	Owner           string   `json:"owner"`
	MakerAddress    string   `json:"maker_address"`
	Market          string   `json:"market"`
	AssetID         string   `json:"asset_id"`
	Side            string   `json:"side"`
	OriginalSize    string   `json:"original_size"`
	SizeMatched     string   `json:"size_matched"`
	Price           string   `json:"price"`
	AssociateTrades []string `json:"associate_trades"`
	Outcome         string   `json:"outcome"`
	CreatedAt       int64    `json:"created_at"`
	Expiration      string   `json:"expiration"`
	OrderType       string   `json:"order_type"`
}

// MakerOrder is the maker-side component of a trade.
type MakerOrder struct {
	OrderID       string `json:"order_id"`
	Owner         string `json:"owner"`
	MakerAddress  string `json:"maker_address"`
	MatchedAmount string `json:"matched_amount"`
	Price         string `json:"price"`
	FeeRateBps    string `json:"fee_rate_bps"`
	AssetID       string `json:"asset_id"`
	Outcome       string `json:"outcome"`
	Side          Side   `json:"side"`
}

// Trade is an authenticated user trade record.
type Trade struct {
	ID              string       `json:"id"`
	TakerOrderID    string       `json:"taker_order_id"`
	Market          string       `json:"market"`
	AssetID         string       `json:"asset_id"`
	Side            Side         `json:"side"`
	Size            string       `json:"size"`
	FeeRateBps      string       `json:"fee_rate_bps"`
	Price           string       `json:"price"`
	Status          string       `json:"status"`
	MatchTime       string       `json:"match_time"`
	LastUpdate      string       `json:"last_update"`
	Outcome         string       `json:"outcome"`
	BucketIndex     int64        `json:"bucket_index"`
	Owner           string       `json:"owner"`
	MakerAddress    string       `json:"maker_address"`
	MakerOrders     []MakerOrder `json:"maker_orders"`
	TransactionHash string       `json:"transaction_hash"`
	TraderSide      string       `json:"trader_side"`
	ErrorMsg        string       `json:"error_msg,omitempty"`
}

// OrderArgs contains the inputs for building a limit order.
type OrderArgs struct {
	TokenID    string
	Price      float64
	Size       float64
	Side       Side
	FeeRateBps int64
	Nonce      uint64
	Expiration uint64
	Taker      string
}

// MarketOrderArgs contains the inputs for building a market order.
type MarketOrderArgs struct {
	TokenID    string
	Amount     float64
	Side       Side
	Price      float64
	FeeRateBps int64
	Nonce      uint64
	Taker      string
	OrderType  OrderType
}

// SignedOrder is the Polymarket wire format for a signed order payload.
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

// MarshalJSON encodes the signed order with the salt as a JSON number.
func (o SignedOrder) MarshalJSON() ([]byte, error) {
	salt, err := strconv.ParseUint(o.Salt, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order salt: %w", err)
	}

	type wireSignedOrder struct {
		Salt          uint64        `json:"salt"`
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

	return json.Marshal(wireSignedOrder{
		Salt:          salt,
		Maker:         o.Maker,
		Signer:        o.Signer,
		Taker:         o.Taker,
		TokenID:       o.TokenID,
		MakerAmount:   o.MakerAmount,
		TakerAmount:   o.TakerAmount,
		Expiration:    o.Expiration,
		Nonce:         o.Nonce,
		FeeRateBps:    o.FeeRateBps,
		Side:          o.Side,
		SignatureType: o.SignatureType,
		Signature:     o.Signature,
	})
}

// PostOrderRequest is the authenticated order-post payload.
type PostOrderRequest struct {
	Order     SignedOrder `json:"order"`
	Owner     string      `json:"owner"`
	OrderType OrderType   `json:"orderType"`
	DeferExec bool        `json:"deferExec"`
	PostOnly  bool        `json:"postOnly,omitempty"`
}

// OpenOrderParams filters authenticated open-order queries.
type OpenOrderParams struct {
	ID      string
	Market  string
	AssetID string
}

// TradeParams filters authenticated trade queries.
type TradeParams struct {
	ID           string
	MakerAddress string
	Market       string
	AssetID      string
	Before       string
	After        string
}

// UnmarshalJSON implements the json.Unmarshaler interface to handle
// the diverse key formats returned by the API for CancelOrdersResponse.
func (r *CancelOrdersResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		Canceled     []string          `json:"canceled"`
		NotCanceled  map[string]string `json:"not_canceled"`
		NotCanceled2 map[string]string `json:"notCanceled"`
	}

	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	r.Canceled = decoded.Canceled
	if decoded.NotCanceled != nil {
		r.NotCanceled = decoded.NotCanceled
	} else {
		r.NotCanceled = decoded.NotCanceled2
	}
	if r.NotCanceled == nil {
		r.NotCanceled = map[string]string{}
	}

	return nil
}
