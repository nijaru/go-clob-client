package clob

// SplitArgs represents the arguments for a split operation.
type SplitArgs struct {
	ConditionID string `json:"conditionId"`
	Amount      string `json:"amount"`
}

// MergeArgs represents the arguments for a merge operation.
type MergeArgs struct {
	ConditionID string `json:"conditionId"`
	Amount      string `json:"amount"`
}

// RedeemArgs represents the arguments for a redeem operation.
type RedeemArgs struct {
	ConditionID string `json:"conditionId"`
}
