package gamma

// MarketClarification is an official note that resolves ambiguity in how a
// market settles.
type MarketClarification struct {
	ID               int                       `json:"id"`
	MarketID         FlexibleID                `json:"marketId,omitzero"`
	EventID          FlexibleID                `json:"eventId,omitzero"`
	QuestionID       *string                   `json:"questionId,omitzero"`
	Content          *string                   `json:"content,omitzero"`
	State            *MarketClarificationState `json:"state,omitzero"`
	ClearBook        *bool                     `json:"clearBook,omitzero"`
	NotifyDiscord    *bool                     `json:"notifyDiscord,omitzero"`
	ShowInFrontend   *bool                     `json:"showInFrontend,omitzero"`
	AdapterAddress   *string                   `json:"adapterAddress,omitzero"`
	NegRiskRequestID *string                   `json:"negRiskRequestId,omitzero"`
	TxHash           *string                   `json:"txHash,omitzero"`
	CreatedAt        Timestamp                 `json:"createdAt,omitzero"`
	QueuedAt         Timestamp                 `json:"queuedAt,omitzero"`
	ScheduledFor     Timestamp                 `json:"scheduledFor,omitzero"`
	TxTimestamp      Timestamp                 `json:"txTimestamp,omitzero"`
}

// MarketClarificationState is the lifecycle state of a market clarification.
type MarketClarificationState string

const (
	MarketClarificationStatePending   MarketClarificationState = "pending"
	MarketClarificationStateQueued    MarketClarificationState = "queued"
	MarketClarificationStateSuccess   MarketClarificationState = "success"
	MarketClarificationStateFailed    MarketClarificationState = "failed"
	MarketClarificationStateCancelled MarketClarificationState = "cancelled"
)
