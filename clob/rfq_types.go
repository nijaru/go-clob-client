package clob

import "github.com/quagmt/udecimal"

// RFQRequest identifies a single Request for Quote.
type RFQRequest struct {
	ID        string `json:"id"`
	AssetIn   string `json:"asset_in"`
	AssetOut  string `json:"asset_out"`
	AmountIn  string `json:"amount_in"`
	AmountOut string `json:"amount_out"`
	UserType  string `json:"user_type"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// RFQQuote identifies a single quote responded to an RFQ request.
type RFQQuote struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
	AssetIn   string `json:"asset_in"`
	AssetOut  string `json:"asset_out"`
	AmountIn  string `json:"amount_in"`
	AmountOut string `json:"amount_out"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

const (
	RFQStatusActive   = "active"
	RFQStatusInactive = "inactive"
)

// CreateRFQRequestParams contains the inputs for creating a new RFQ request.
type CreateRFQRequestParams struct {
	AssetIn   string           `json:"asset_in"`
	AssetOut  string           `json:"asset_out"`
	AmountIn  udecimal.Decimal `json:"amount_in"`
	AmountOut udecimal.Decimal `json:"amount_out"`
	UserType  string           `json:"user_type"` // "EOA" or "POLY_PROXY"
}

// CreateRFQQuoteParams contains the inputs for a quoter to respond to an RFQ request.
type CreateRFQQuoteParams struct {
	RequestID string           `json:"request_id"`
	AssetIn   string           `json:"asset_in"`
	AssetOut  string           `json:"asset_out"`
	AmountIn  udecimal.Decimal `json:"amount_in"`
	AmountOut udecimal.Decimal `json:"amount_out"`
}

// AcceptRFQQuoteRequest is the payload for accepting a specific quote.
type AcceptRFQQuoteRequest struct {
	QuoteID string `json:"quote_id"`
}

// AcceptRFQQuoteResponse is the response for accepting a quote.
// It returns a signed order payload that the requester needs to sign and return.
type AcceptRFQQuoteResponse struct {
	Order SignedOrder `json:"order"`
}

// ApproveRFQOrderRequest is the payload for a quoter to approve an order.
type ApproveRFQOrderRequest struct {
	RequestID string      `json:"request_id"`
	Order     SignedOrder `json:"order"`
}

// RFQRequestsResponse is the response for listing RFQ requests.
type RFQRequestsResponse []RFQRequest

// RFQQuotesResponse is the response for listing RFQ quotes.
type RFQQuotesResponse []RFQQuote

// RFQRequestFilterParams contains the filters for listing RFQ requests.
type RFQRequestFilterParams struct {
	Limit      int
	Offset     string // base64 encoded integer
	State      string // "active" or "inactive"
	RequestIDs []string
}

// RFQQuoteFilterParams contains the filters for listing RFQ quotes.
type RFQQuoteFilterParams struct {
	Limit      int
	Offset     string
	RequestIDs []string
}
