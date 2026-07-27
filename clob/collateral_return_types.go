package clob

import "github.com/ethereum/go-ethereum/common"

// CollateralReturnOperationKind identifies a known operation in a collateral
// return plan. The service may add new kinds; CollateralReturnOperation.Kind is
// intentionally a string so newer plans remain decodable.
type CollateralReturnOperationKind string

const (
	CollateralReturnSplit              CollateralReturnOperationKind = "split"
	CollateralReturnMerge              CollateralReturnOperationKind = "merge"
	CollateralReturnRedeem             CollateralReturnOperationKind = "redeem"
	CollateralReturnSplitOnCondition   CollateralReturnOperationKind = "split_on_condition"
	CollateralReturnMergeOnCondition   CollateralReturnOperationKind = "merge_on_condition"
	CollateralReturnSplitOnEvent       CollateralReturnOperationKind = "split_on_event"
	CollateralReturnMergeOnEvent       CollateralReturnOperationKind = "merge_on_event"
	CollateralReturnConvertOnEvent     CollateralReturnOperationKind = "convert_on_event"
	CollateralReturnExtract            CollateralReturnOperationKind = "extract"
	CollateralReturnInject             CollateralReturnOperationKind = "inject"
	CollateralReturnConvertToYesBasket CollateralReturnOperationKind = "convert_to_yes_basket"
	CollateralReturnMergeFromYesBasket CollateralReturnOperationKind = "merge_from_yes_basket"
	CollateralReturnCompress           CollateralReturnOperationKind = "compress"
)

// CollateralReturnOperation describes one position operation in a plan.
type CollateralReturnOperation struct {
	Kind           CollateralReturnOperationKind `json:"kind"`
	ConditionID    string                        `json:"condition_id,omitzero"`
	EventID        string                        `json:"event_id,omitzero"`
	PositionID     string                        `json:"position_id,omitzero"`
	ConditionIndex int                           `json:"condition_index,omitzero"`
	Amount         string                        `json:"amount"`
}

// CollateralReturnPositionAmount describes a position amount touched by a plan.
type CollateralReturnPositionAmount struct {
	PositionID string `json:"position_id"`
	Amount     string `json:"amount"`
}

// CollateralReturnPositionSummary describes the net position impact of a plan.
type CollateralReturnPositionSummary struct {
	Consumed []CollateralReturnPositionAmount `json:"consumed,omitzero"`
	Created  []CollateralReturnPositionAmount `json:"created,omitzero"`
}

// CollateralReturnRouterCall is the exact on-chain call a plan executes.
type CollateralReturnRouterCall struct {
	To   common.Address `json:"to"`
	Data string         `json:"data"`
}

// CollateralReturnPlan is an inspectable, executable plan returned by the
// collateral-return service. Execute it only after reviewing the value and
// position fields. If Truncated is true, execute this chunk, wait for it to
// settle, then request a fresh plan for the remainder.
type CollateralReturnPlan struct {
	PlanHash             string                           `json:"plan_hash"`
	ChainID              int64                            `json:"chain_id"`
	Wallet               common.Address                   `json:"wallet"`
	BlockNumber          string                           `json:"block_number"`
	StartingPUSD         string                           `json:"starting_pusd"`
	NetPUSDOut           string                           `json:"net_pusd_out"`
	FinalPUSD            string                           `json:"final_pusd"`
	Operations           []CollateralReturnOperation      `json:"operations"`
	OperationCount       int                              `json:"operation_count"`
	Truncated            bool                             `json:"truncated"`
	EstimatedCost        float64                          `json:"estimated_cost"`
	RequiredPUSDInput    string                           `json:"required_pusd_input"`
	RequiredPositions    []CollateralReturnPositionAmount `json:"required_positions"`
	PositionSummary      CollateralReturnPositionSummary  `json:"position_summary"`
	CandidatePositionIDs []string                         `json:"candidate_position_ids"`
	RouterCall           CollateralReturnRouterCall       `json:"router_call"`
}
