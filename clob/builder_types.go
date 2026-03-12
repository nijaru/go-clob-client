package clob

import (
	"context"
	"net/http"
)

// BuilderHeaderRequest contains the request metadata needed to create builder headers.
type BuilderHeaderRequest struct {
	Method    string
	Path      string
	Body      []byte
	Timestamp int64
}

// BuilderAuth produces builder headers for supported requests.
type BuilderAuth interface {
	Headers(ctx context.Context, req BuilderHeaderRequest) (map[string]string, error)
}

// RemoteBuilderAuthConfig configures a remote builder-signing service.
type RemoteBuilderAuthConfig struct {
	URL         string
	BearerToken string
	HTTPClient  *http.Client
}

// BuilderAPIKey is the metadata returned when listing builder API keys.
type BuilderAPIKey struct {
	Key       string `json:"key"`
	CreatedAt string `json:"createdAt,omitempty"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

// BuilderTrade is a builder-specific trade record.
type BuilderTrade struct {
	ID              string `json:"id"`
	TradeType       string `json:"tradeType"`
	TakerOrderHash  string `json:"takerOrderHash"`
	Builder         string `json:"builder"`
	Market          string `json:"market"`
	AssetID         string `json:"assetId"`
	Side            string `json:"side"`
	Size            string `json:"size"`
	SizeUSDC        string `json:"sizeUsdc"`
	Price           string `json:"price"`
	Status          string `json:"status"`
	Outcome         string `json:"outcome"`
	OutcomeIndex    int64  `json:"outcomeIndex"`
	RequestID       string `json:"requestId"`
	Error           string `json:"error,omitempty"`
	Owner           string `json:"owner,omitempty"`
	Maker           string `json:"maker,omitempty"`
	TransactionHash string `json:"transactionHash,omitempty"`
	MatchTime       string `json:"matchTime,omitempty"`
	BucketIndex     int64  `json:"bucketIndex,omitempty"`
	Fee             string `json:"fee,omitempty"`
	FeeUSDC         string `json:"feeUsdc,omitempty"`
	CreatedAt       string `json:"createdAt,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
}

// HeartbeatResponse is the response payload from posting a heartbeat.
type HeartbeatResponse struct {
	HeartbeatID string `json:"heartbeat_id"`
	Error       string `json:"error,omitempty"`
}
