package data

import (
	"encoding/json"
	"fmt"
)

// Activity is a wallet event. Type selects which optional fields are meaningful:
// trades carry AssetID/Side/Shares/Price, market operations carry ConditionID,
// and credits carry Amount. IsCombo distinguishes combo trades from market trades.
// Unknown types preserve their complete wire payload in Raw instead of vanishing.
type Activity struct {
	Type                  ActivityType    `json:"type"`
	Wallet                string          `json:"proxy_wallet"`
	Timestamp             Timestamp       `json:"timestamp"`
	TransactionHash       string          `json:"transaction_hash"`
	Name                  *string         `json:"name"`
	Pseudonym             *string         `json:"pseudonym"`
	Bio                   *string         `json:"bio"`
	ProfileImage          *string         `json:"profile_image"`
	ProfileImageOptimized *string         `json:"profile_image_optimized"`
	IsCombo               bool            `json:"is_combo"`
	ConditionID           *string         `json:"condition_id"`
	AssetID               *string         `json:"token_id"`
	Side                  *string         `json:"side"`
	Shares                *DecimalString  `json:"size"`
	Amount                *DecimalString  `json:"usdc_size"`
	Price                 *DecimalString  `json:"price"`
	Outcome               *string         `json:"outcome"`
	OutcomeIndex          *int64          `json:"outcome_index"`
	Title                 *string         `json:"title"`
	Slug                  *string         `json:"slug"`
	Icon                  *string         `json:"icon"`
	EventSlug             *string         `json:"event_slug"`
	Raw                   json.RawMessage `json:"-"`
}

func (a *Activity) UnmarshalJSON(raw []byte) error {
	if len(raw) == 0 || raw[0] != '{' {
		return fmt.Errorf("data: activity must be an object")
	}
	type wire Activity
	var value wire
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	known := false
	switch value.Type {
	case ActivityTypeTrade, ActivityTypeSplit, ActivityTypeMerge, ActivityTypeRedeem,
		ActivityTypeReward, ActivityTypeConversion, ActivityTypeMigration,
		ActivityTypeDeposit, ActivityTypeWithdrawal, ActivityTypeYield,
		ActivityTypeMakerRebate, ActivityTypeTakerRebate, ActivityTypeReferralReward, ActivityTypeTip:
		known = true
	}
	if known {
		var required struct {
			Wallet          *string    `json:"proxy_wallet"`
			Timestamp       *Timestamp `json:"timestamp"`
			TransactionHash *string    `json:"transaction_hash"`
		}
		if err := json.Unmarshal(raw, &required); err != nil {
			return err
		}
		if required.Wallet == nil || required.Timestamp == nil || required.TransactionHash == nil {
			return fmt.Errorf("data: incomplete %s activity", value.Type)
		}
		if value.Amount == nil {
			return fmt.Errorf("data: %s activity is missing usdc_size", value.Type)
		}
		switch value.Type {
		case ActivityTypeSplit, ActivityTypeMerge, ActivityTypeRedeem, ActivityTypeConversion:
			if value.ConditionID == nil {
				return fmt.Errorf("data: %s activity is missing condition_id", value.Type)
			}
		}
		if value.Type == ActivityTypeTrade &&
			(value.ConditionID == nil || value.AssetID == nil || value.Side == nil || value.Shares == nil || value.Amount == nil || value.Price == nil) {
			return fmt.Errorf("data: incomplete trade activity")
		}
	} else {
		value.Raw = append(json.RawMessage(nil), raw...)
	}
	if value.OutcomeIndex != nil && *value.OutcomeIndex == 999 {
		value.OutcomeIndex = nil
	}
	clearEmptyText(
		&value.Name,
		&value.Pseudonym,
		&value.Bio,
		&value.ProfileImage,
		&value.ProfileImageOptimized,
		&value.Title,
		&value.Slug,
		&value.Icon,
		&value.EventSlug,
		&value.Outcome,
	)
	*a = Activity(value)
	return nil
}

// ComboActivity is a combo lifecycle operation. Payout is present on redemption.
type ComboActivity struct {
	ID              string             `json:"id"                 required:"true"`
	Type            ComboActivityType  `json:"type"               required:"true"`
	Wallet          string             `json:"proxy_wallet"       required:"true"`
	ConditionID     string             `json:"combo_condition_id" required:"true"`
	PositionID      string             `json:"combo_position_id"  required:"true"`
	Amount          *DecimalString     `json:"amount_usdc"`
	Payout          *DecimalString     `json:"payout_usdc"`
	Timestamp       Timestamp          `json:"timestamp"          required:"true"`
	TransactionHash string             `json:"transaction_hash"   required:"true"`
	BlockNumber     int64              `json:"block_number"       required:"true"`
	Legs            []ComboPositionLeg `json:"legs"               required:"true"`
}

func (a *ComboActivity) UnmarshalJSON(raw []byte) error {
	type wire ComboActivity
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	switch value.Type {
	case ComboActivityTypeSplit, ComboActivityTypeMerge, ComboActivityTypeConvert,
		ComboActivityTypeCompress, ComboActivityTypeWrap, ComboActivityTypeUnwrap, ComboActivityTypeRedeem:
	default:
		return fmt.Errorf("data: unknown combo activity type %q", value.Type)
	}
	*a = ComboActivity(value)
	return nil
}

// BiggestWinner is a resolved market or combo position. AssetID is the wire
// position_id for either kind; EventID/EventSlug are present only for markets.
// Equal profits retain distinct row ordinals, unlike trader leaderboard ranks.
type BiggestWinner struct {
	Kind         BiggestWinnerKind `json:"kind"          required:"true"`
	Rank         int64             `json:"win_rank"      required:"true"`
	Wallet       string            `json:"user_id"       required:"true"`
	PnL          DecimalString     `json:"pnl"           required:"true"`
	InitialValue DecimalString     `json:"initial_value" required:"true"`
	FinalValue   DecimalString     `json:"final_value"   required:"true"`
	ResolvedAt   Timestamp         `json:"resolved_at"   required:"true"`
	UserName     *string           `json:"user_name"`
	ProfileImage *string           `json:"profile_image"`
	EventTitle   *string           `json:"event_title"`
	ConditionID  string            `json:"condition_id"  required:"true"`
	AssetID      string            `json:"position_id"   required:"true"`
	EventID      *EventID          `json:"event_id"`
	EventSlug    *string           `json:"event_slug"`
}

func (b *BiggestWinner) UnmarshalJSON(raw []byte) error {
	type wire BiggestWinner
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	if value.Kind != BiggestWinnerKindMarket && value.Kind != BiggestWinnerKindCombo {
		return fmt.Errorf("data: unknown biggest winner kind %q", value.Kind)
	}
	if value.Kind == BiggestWinnerKindMarket &&
		(value.EventID == nil || *value.EventID == "" || *value.EventID == "0") {
		return fmt.Errorf("data: market winner is missing event_id")
	}
	*b = BiggestWinner(value)
	return nil
}
