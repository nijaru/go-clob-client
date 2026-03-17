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
	CreatedAt string `json:"createdAt,omitzero"`
	RevokedAt string `json:"revokedAt,omitzero"`
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
	Error           string `json:"error,omitzero"`
	Owner           string `json:"owner,omitzero"`
	Maker           string `json:"maker,omitzero"`
	TransactionHash string `json:"transactionHash,omitzero"`
	MatchTime       string `json:"matchTime,omitzero"`
	BucketIndex     int64  `json:"bucketIndex,omitzero"`
	Fee             string `json:"fee,omitzero"`
	FeeUSDC         string `json:"feeUsdc,omitzero"`
	CreatedAt       string `json:"createdAt,omitzero"`
	UpdatedAt       string `json:"updatedAt,omitzero"`
}
