package clob

import (
	"fmt"
	"strconv"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/quagmt/udecimal"
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
	// RebateEstimated is the projected maker rebate for this order (2026 fee schedule).
	RebateEstimated string `json:"rebate_estimated,omitzero"`
}

// UnmarshalJSON accepts the field name variants the live API returns for post-order responses.
func (r *PostOrderResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	if v, ok := fields["success"]; ok {
		json.Unmarshal(v, &r.Success)
	}
	if v, ok := fields["status"]; ok {
		json.Unmarshal(v, &r.Status)
	}
	if v, ok := fields["takingAmount"]; ok {
		json.Unmarshal(v, &r.TakingAmount)
	}
	if v, ok := fields["makingAmount"]; ok {
		json.Unmarshal(v, &r.MakingAmount)
	}

	if val, ok, err := decodeStringAlias(fields, "orderID", "order_id"); err != nil {
		return err
	} else if ok {
		r.OrderID = val
	}

	if val, ok, err := decodeStringAlias(fields, "errorMsg", "error_msg"); err != nil {
		return err
	} else if ok {
		r.ErrorMsg = val
	}

	if val, ok, err := decodeStringAlias(fields, "rebate_estimated", "rebateEstimated"); err != nil {
		return err
	} else if ok {
		r.RebateEstimated = val
	}

	if val, ok, err := decodeStringSliceAlias(fields, "transactionsHashes", "transaction_hashes"); err != nil {
		return err
	} else if ok {
		r.TransactionsHashes = val
	}

	if val, ok, err := decodeStringSliceAlias(fields, "trade_ids", "tradeIds"); err != nil {
		return err
	} else if ok {
		r.TradeIDs = val
	}

	return nil
}

func decodeStringAlias(fields map[string]jsontext.Value, keys ...string) (string, bool, error) {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		if string(value) == "null" {
			return "", true, nil
		}

		var out string
		if err := json.Unmarshal(value, &out); err != nil {
			return "", false, fmt.Errorf("decode %s: %w", key, err)
		}
		return out, true, nil
	}

	return "", false, nil
}

func decodeStringSliceAlias(
	fields map[string]jsontext.Value,
	keys ...string,
) ([]string, bool, error) {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		if string(value) == "null" {
			return []string{}, true, nil
		}

		var out []string
		if err := json.Unmarshal(value, &out); err != nil {
			return nil, false, fmt.Errorf("decode %s: %w", key, err)
		}
		return out, true, nil
	}

	return nil, false, nil
}

// CancelOrdersResponse reports which orders were canceled successfully.
type CancelOrdersResponse struct {
	Canceled    []string          `json:"canceled"`
	NotCanceled map[string]string `json:"notCanceled"`
}

// AmountKind specifies how Amount is interpreted in a market order.
type AmountKind uint8

const (
	// AmountUSDC treats Amount as a USDC value to spend (buys only).
	// The number of shares purchased is computed from the market price.
	// This is the default (zero value) and matches the current behavior for SideBuy.
	AmountUSDC AmountKind = iota
	// AmountShares treats Amount as a number of shares.
	// For SideSell: always use this. For SideBuy: buy exactly Amount shares,
	// spending Amount * price USDC.
	AmountShares
)

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
	ErrorMsg        string       `json:"error_msg,omitzero"`
	// RebateEstimated is the projected maker rebate for this trade.
	RebateEstimated string `json:"rebate_estimated,omitzero"`
}

// OrderArgs contains the inputs for building a limit order.
type OrderArgs struct {
	TokenID    string
	Price      udecimal.Decimal
	Size       udecimal.Decimal
	Side       Side
	FeeRateBps int64
	Nonce      uint64
	Expiration uint64
	Taker      string
}

// MarketOrderArgs contains the inputs for building a market order.
type MarketOrderArgs struct {
	TokenID string
	// Amount is the quantity to trade. Its interpretation depends on AmountKind:
	// AmountUSDC (default): for SideBuy, spend exactly Amount USDC; shares are derived from price.
	// AmountShares: for SideBuy, buy exactly Amount shares, spending Amount*price USDC;
	//               for SideSell, sell exactly Amount shares.
	Amount     udecimal.Decimal
	AmountKind AmountKind
	Side       Side
	Price      udecimal.Decimal
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

// UnmarshalJSON handles the diverse salt format from the API.
func (o *SignedOrder) UnmarshalJSON(data []byte) error {
	type wireSignedOrder struct {
		Salt          any           `json:"salt"`
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

	var decoded wireSignedOrder
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	o.Salt = fmt.Sprint(decoded.Salt)
	o.Maker = decoded.Maker
	o.Signer = decoded.Signer
	o.Taker = decoded.Taker
	o.TokenID = decoded.TokenID
	o.MakerAmount = decoded.MakerAmount
	o.TakerAmount = decoded.TakerAmount
	o.Expiration = decoded.Expiration
	o.Nonce = decoded.Nonce
	o.FeeRateBps = decoded.FeeRateBps
	o.Side = decoded.Side
	o.SignatureType = decoded.SignatureType
	o.Signature = decoded.Signature
	return nil
}

// PostOrderRequest is the authenticated order-post payload.
type PostOrderRequest struct {
	Order     SignedOrder `json:"order"`
	Owner     string      `json:"owner"`
	OrderType OrderType   `json:"orderType"`
	PostOnly  bool        `json:"postOnly,omitzero"`
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
	TakerAddress string
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
