package data

import (
	"context"
	"net/url"
	"slices"
	"strings"
)

// TokenModule identifies the contract family that issued an outcome asset.
type TokenModule string

const (
	// TokenModuleV1CTF marks a legacy CTF token.
	TokenModuleV1CTF TokenModule = "v1_ctf"
	// TokenModuleBinary marks a binary V2 position.
	TokenModuleBinary TokenModule = "binary"
	// TokenModuleNegRisk marks a neg-risk V2 position.
	TokenModuleNegRisk TokenModule = "neg_risk"
	// TokenModuleCombo marks a combo basket token.
	TokenModuleCombo TokenModule = "combo"
)

// TokenReference is reference and settlement metadata for one outcome asset.
// Optional fields are present on the wire and may be null. Outcome labels and
// settlement metadata can lag by up to a minute. CLOBIndex is the market's
// order-book slot and can differ from the on-chain OutcomeIndex. EventID is the
// lowest associated event ID.
type TokenReference struct {
	AssetID               string         `json:"token_id"                required:"true"`
	ConditionID           string         `json:"condition_id"            required:"true"`
	StructuralConditionID *string        `json:"structural_condition_id"`
	Module                TokenModule    `json:"module"                  required:"true"`
	OutcomeIndex          int64          `json:"outcome_index"           required:"true"`
	CLOBIndex             *int64         `json:"clob_index"`
	Outcome               *string        `json:"outcome"`
	OppositeAssetID       *string        `json:"opposite_token_id"`
	Resolved              bool           `json:"resolved"                required:"true"`
	FinalPrice            *DecimalString `json:"final_price"`
	Title                 *string        `json:"title"`
	MarketSlug            *string        `json:"market_slug"`
	Closed                *bool          `json:"closed"`
	NegRisk               *bool          `json:"neg_risk"`
	NegRiskMarketID       *string        `json:"neg_risk_market_id"`
	QuestionIndex         *int64         `json:"question_index"`
	EventID               *EventID       `json:"event_id"`
	EventSlug             *string        `json:"event_slug"`
}

// TokenReferenceSelectors selects outcome assets by decimal asset IDs or by
// condition IDs, never both. Blank selectors are dropped. Asset IDs have their
// leading zeros stripped; condition IDs may be 31 or 32 bytes and are compared
// case-insensitively.
type TokenReferenceSelectors struct {
	AssetIDs     []string
	ConditionIDs []string
}

const (
	maxTokenAssetSelectors     = 50
	maxTokenConditionSelectors = 10
	maxTokenAssetDigits        = 78
)

// GetTokenReferences returns reference metadata for the selected outcome
// assets. Unknown selectors contribute no rows; asset lookups retain request
// order while condition groups retain server order, so overlapping conditions
// can repeat an asset.
func (c *Client) GetTokenReferences(
	ctx context.Context,
	selectors TokenReferenceSelectors,
) ([]TokenReference, error) {
	query := url.Values{}
	switch {
	case selectors.AssetIDs != nil && selectors.ConditionIDs != nil:
		return nil, input("selectors", "choose asset IDs or condition IDs, not both")
	case selectors.AssetIDs != nil:
		ids, err := tokenAssetIDs(selectors.AssetIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			query.Add("token_id", id)
		}
	case selectors.ConditionIDs != nil:
		ids, err := tokenConditionIDs(selectors.ConditionIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			query.Add("condition", id)
		}
	default:
		return nil, input("selectors", "provide asset IDs or condition IDs")
	}
	return getList[TokenReference](ctx, c, "/v2/tokens", query)
}

func tokenAssetIDs(values []string) ([]string, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Trim(value, "0123456789") != "" {
			return nil, input("asset_ids", "must contain decimal digit strings")
		}
		value = strings.TrimLeft(value, "0")
		if value == "" {
			value = "0"
		}
		if len(value) > maxTokenAssetDigits {
			return nil, input(
				"asset_ids",
				"must contain at most 78 digits after leading zeros",
			)
		}
		if !slices.Contains(ids, value) {
			ids = append(ids, value)
		}
	}
	if len(ids) == 0 {
		return nil, input("asset_ids", "must contain at least one selector")
	}
	if len(ids) > maxTokenAssetSelectors {
		return nil, input("asset_ids", "accepts at most 50 distinct values")
	}
	return ids, nil
}

func tokenConditionIDs(values []string) ([]string, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized, ok := normalizeTokenConditionID(value)
		if !ok {
			return nil, input("condition_ids", "must contain 31-byte or 32-byte hex strings")
		}
		if !slices.Contains(ids, normalized) {
			ids = append(ids, normalized)
		}
	}
	if len(ids) == 0 {
		return nil, input("condition_ids", "must contain at least one selector")
	}
	if len(ids) > maxTokenConditionSelectors {
		return nil, input("condition_ids", "accepts at most 10 distinct values")
	}
	return ids, nil
}

// normalizeTokenConditionID accepts the unpadded 31-byte form and the
// zero-padded 32-byte form without changing their width.
func normalizeTokenConditionID(value string) (string, bool) {
	// 0x plus 62 or 64 hex digits.
	if len(value) != 64 && len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return "", false
	}
	for _, digit := range value[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", digit) {
			return "", false
		}
	}
	return strings.ToLower(value), true
}
