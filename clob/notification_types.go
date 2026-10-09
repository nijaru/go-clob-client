package clob

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/nijaru/go-clob-client/gamma"
)

// Notification is a Polymarket user notification. It is a discriminated
// union on Type: narrowing on Type also narrows Payload to the typed shape
// carried by that notification kind. Notifications whose Type is unknown to
// this SDK are skipped by GetNotifications so newly introduced kinds cannot
// fail the feed; recognized kinds whose payloads do not match their schemas
// reject the entire page.
type Notification struct {
	Type      NotificationType    `json:"type"`
	Owner     string              `json:"owner"`
	Payload   NotificationPayload `json:"payload"`
	ID        int64               `json:"id"`
	Timestamp int64               `json:"timestamp"`
}

// NotificationType identifies a notification kind. The wire value is an
// integer discriminant; each kind carries a payload whose shape is tied to
// the kind.
type NotificationType int

const (
	NotificationOrderCancellation NotificationType = 1
	NotificationOrderFill         NotificationType = 2
	NotificationMarketRegistered  NotificationType = 3
	NotificationMarketResolved    NotificationType = 4
	NotificationRewardPayout      NotificationType = 5
	NotificationChildComment      NotificationType = 6
	NotificationYieldPayout       NotificationType = 7
	NotificationOrderFillFailed   NotificationType = 8
	NotificationAutoRedeemed      NotificationType = 9
	NotificationComboAutoRedeemed NotificationType = 10
)

// NotificationPayload is a discriminated union keyed by Notification.Type.
// Exactly one variant is non-nil for a valid notification.
type NotificationPayload struct {
	OrderCancellation *OrderNotificationPayload
	OrderFill         *OrderNotificationPayload
	MarketRegistered  *MarketNotificationPayload
	MarketResolved    *MarketNotificationPayload
	RewardPayout      *RewardPayoutNotificationPayload
	ChildComment      *ChildCommentNotificationPayload
	YieldPayout       *YieldPayoutNotificationPayload
	OrderFillFailed   *OrderNotificationPayload
	AutoRedeemed      *AutoRedeemedNotificationPayload
	ComboAutoRedeemed *ComboAutoRedeemedNotificationPayload
}

// OrderNotificationPayload is the payload of an order lifecycle notification
// (cancellation, fill, and failed fill share this shape).
type OrderNotificationPayload struct {
	AssetID         string        `json:"asset_id"`
	ConditionID     string        `json:"market"`
	OrderID         string        `json:"order_id"`
	Side            Side          `json:"side"`
	OrderType       string        `json:"type,omitempty"`
	Price           DecimalString `json:"price"`
	OriginalSize    DecimalString `json:"original_size"`
	MatchedSize     DecimalString `json:"matched_size"`
	RemainingSize   DecimalString `json:"remaining_size"`
	Outcome         string        `json:"outcome"`
	OutcomeIndex    int64         `json:"outcome_index"`
	TransactionHash string        `json:"transaction_hash,omitempty"`
	TradeID         string        `json:"trade_id,omitempty"`
	Question        string        `json:"question,omitempty"`
	MarketSlug      string        `json:"market_slug,omitempty"`
	Icon            string        `json:"icon,omitempty"`
	Image           string        `json:"image,omitempty"`
	EventSlug       string        `json:"eventSlug,omitempty"`
	SeriesSlug      string        `json:"seriesSlug,omitempty"`
}

// UnmarshalJSON accepts the asset ID under the legacy token_id spelling in
// addition to asset_id (py-sdk AliasChoices parity).
func (p *OrderNotificationPayload) UnmarshalJSON(data []byte) error {
	type alias OrderNotificationPayload
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.AssetID == "" {
		var legacy struct {
			AssetID string `json:"token_id"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		value.AssetID = legacy.AssetID
	}
	*p = OrderNotificationPayload(value)
	return nil
}

// MarketNotificationToken is one outcome token inside a market lifecycle
// notification payload. On a market-resolved notification, Winner marks the
// winning outcome.
type MarketNotificationToken struct {
	TokenID string        `json:"token_id"`
	Outcome string        `json:"outcome"`
	Price   DecimalString `json:"price,omitempty"`
	Winner  bool          `json:"winner"`
}

// UnmarshalJSON accepts the asset ID under the asset_id spelling in addition
// to the legacy token_id key (py-sdk AliasChoices parity).
func (t *MarketNotificationToken) UnmarshalJSON(data []byte) error {
	type alias MarketNotificationToken
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.TokenID == "" {
		var current struct {
			AssetID string `json:"asset_id"`
		}
		if err := json.Unmarshal(data, &current); err != nil {
			return err
		}
		value.TokenID = current.AssetID
	}
	*t = MarketNotificationToken(value)
	return nil
}

// MarketNotificationRewardsRate is one per-asset daily reward rate carried on
// a market lifecycle notification.
type MarketNotificationRewardsRate struct {
	AssetAddress string        `json:"asset_address"`
	DailyRate    DecimalString `json:"rewards_daily_rate"`
}

// MarketNotificationRewards carries liquidity-rewards parameters on a market
// lifecycle notification.
type MarketNotificationRewards struct {
	MinSize   DecimalString                   `json:"min_size"`
	MaxSpread DecimalString                   `json:"max_spread"`
	Rates     []MarketNotificationRewardsRate `json:"rates,omitempty"`
}

// MarketNotificationPayload is the payload of a market lifecycle notification
// (market registered and market resolved share this shape).
type MarketNotificationPayload struct {
	ConditionID              string                     `json:"condition_id"`
	QuestionID               string                     `json:"question_id"`
	Question                 string                     `json:"question"`
	Description              string                     `json:"description"`
	MarketSlug               string                     `json:"market_slug"`
	Icon                     string                     `json:"icon"`
	Image                    string                     `json:"image"`
	Fpmm                     string                     `json:"fpmm"`
	Active                   bool                       `json:"active"`
	Closed                   bool                       `json:"closed"`
	Archived                 *bool                      `json:"archived,omitempty"`
	AcceptingOrders          bool                       `json:"accepting_orders"`
	AcceptingOrdersTimestamp *time.Time                 `json:"accepting_order_timestamp,omitempty"`
	EnableOrderBook          *bool                      `json:"enable_order_book,omitempty"`
	EndDate                  *time.Time                 `json:"end_date_iso,omitempty"`
	GameStartTime            *time.Time                 `json:"game_start_time,omitempty"`
	SecondsDelay             int                        `json:"seconds_delay"`
	MinimumOrderSize         DecimalString              `json:"minimum_order_size"`
	MinimumTickSize          DecimalString              `json:"minimum_tick_size"`
	MakerBaseFee             *DecimalString             `json:"maker_base_fee,omitempty"`
	TakerBaseFee             *DecimalString             `json:"taker_base_fee,omitempty"`
	NotificationsEnabled     *bool                      `json:"notifications_enabled,omitempty"`
	NegRisk                  *bool                      `json:"neg_risk,omitempty"`
	NegRiskMarketID          string                     `json:"neg_risk_market_id,omitempty"`
	NegRiskRequestID         string                     `json:"neg_risk_request_id,omitempty"`
	Is5050Outcome            *bool                      `json:"is_50_50_outcome,omitempty"`
	Rewards                  *MarketNotificationRewards `json:"rewards,omitempty"`
	Tokens                   []MarketNotificationToken  `json:"tokens"`
	Tags                     []string                   `json:"tags,omitempty"`
	EventSlug                string                     `json:"eventSlug,omitempty"`
}

// RewardPayoutNotificationPayload is the payload of a liquidity-reward
// payout notification.
type RewardPayoutNotificationPayload struct {
	ProxyWallet     string        `json:"proxyWallet"`
	Reward          DecimalString `json:"reward"`
	TransactionHash string        `json:"txnHash"`
}

// YieldPayoutNotificationPayload is the payload of a yield payout notification.
type YieldPayoutNotificationPayload struct {
	ProxyWallet     string        `json:"proxyWallet"`
	Amount          DecimalString `json:"amount"`
	TransactionHash string        `json:"txnHash"`
}

// ChildCommentNotificationPayload is the payload of a child-comment
// notification: the reply comment, its author's profile, and the event or
// series the thread belongs to.
type ChildCommentNotificationPayload struct {
	Profile          *gamma.CommentProfile  `json:"profile,omitempty"`
	ID               NotificationCommentID  `json:"id"`
	Body             *string                `json:"body,omitempty"`
	ParentEntityType *string                `json:"parentEntityType,omitempty"`
	ParentEntityID   *int64                 `json:"parentEntityID,omitempty"`
	ParentCommentID  *NotificationCommentID `json:"parentCommentID,omitempty"`
	UserAddress      *string                `json:"userAddress,omitempty"`
	CreatedAt        *time.Time             `json:"createdAt,omitempty"`
	EventSlug        *string                `json:"eventSlug,omitempty"`
	EventTitle       *string                `json:"eventTitle,omitempty"`
	SeriesSlug       *string                `json:"seriesSlug,omitempty"`
	SeriesTitle      *string                `json:"seriesTitle,omitempty"`
	Image            *string                `json:"image,omitempty"`
}

// AutoRedeemedNotificationPayload is the payload of an auto-redeem
// notification: a winning position redeemed on-chain on the account's behalf.
type AutoRedeemedNotificationPayload struct {
	ProxyWallet     string        `json:"proxyWallet"`
	Amount          DecimalString `json:"amount"`
	ConditionID     string        `json:"conditionId"`
	Question        string        `json:"question"`
	Image           string        `json:"image"`
	MarketSlug      string        `json:"slug"`
	Position        *string       `json:"position,omitempty"`
	MarketURL       *string       `json:"marketUrl,omitempty"`
	PortfolioURL    *string       `json:"portfolioUrl,omitempty"`
	NegRisk         bool          `json:"negRisk"`
	TransactionHash string        `json:"txnHash"`
}

// ComboAutoRedeemedNotificationPayload is the payload of a combo auto-redeem
// notification: a winning combo position redeemed on-chain on the account's
// behalf. Legs is the combo arity.
type ComboAutoRedeemedNotificationPayload struct {
	ProxyWallet     string        `json:"proxyWallet"`
	Amount          DecimalString `json:"amount"`
	PositionID      string        `json:"positionId"`
	ConditionID     string        `json:"conditionId"`
	OutcomeIndex    int           `json:"outcomeIndex"`
	Legs            int           `json:"legs"`
	PortfolioURL    *string       `json:"portfolioUrl,omitempty"`
	TransactionHash string        `json:"txnHash"`
}

// errUnknownNotificationType is returned by Notification.UnmarshalJSON when
// the wire type is not one of the known notification kinds. GetNotifications
// skips these so newly introduced kinds cannot fail the feed.
var errUnknownNotificationType = errors.New("unknown notification type")

// probeNotificationType reads only the "type" field from a notification
// JSON object. It returns the raw integer value and whether the field was
// present and an integer.
func probeNotificationType(data []byte) (int, bool, error) {
	var probe struct {
		Type stdjson.Number `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, false, err
	}
	if probe.Type == "" {
		return 0, false, nil
	}
	value, err := probe.Type.Int64()
	if err != nil {
		return 0, false, nil
	}
	return int(value), true, nil
}

// UnmarshalJSON decodes a notification, discriminating on the integer "type"
// field. Unknown types return errUnknownNotificationType so list pages can
// skip them; known types with malformed payloads reject the notification.
// Optional metadata retains nil separately from an explicit empty value.
func (n *Notification) UnmarshalJSON(data []byte) error {
	typeVal, ok, err := probeNotificationType(data)
	if err != nil {
		return fmt.Errorf("probe notification type: %w", err)
	}
	if !ok {
		return fmt.Errorf("notification missing type field")
	}

	// A reused destination must never retain a different payload variant.
	*n = Notification{}
	ntype := NotificationType(typeVal)
	switch ntype {
	case NotificationOrderCancellation, NotificationOrderFill, NotificationOrderFillFailed:
		id, owner, ts, order, err := decodeNotificationPayload[OrderNotificationPayload](data)
		if err != nil {
			return err
		}
		switch ntype {
		case NotificationOrderCancellation:
			n.Payload.OrderCancellation = order
		case NotificationOrderFill:
			n.Payload.OrderFill = order
		default:
			n.Payload.OrderFillFailed = order
		}
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationMarketRegistered, NotificationMarketResolved:
		id, owner, ts, market, err := decodeNotificationPayload[MarketNotificationPayload](data)
		if err != nil {
			return err
		}
		if ntype == NotificationMarketRegistered {
			n.Payload.MarketRegistered = market
		} else {
			n.Payload.MarketResolved = market
		}
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationRewardPayout:
		id, owner, ts, reward, err := decodeNotificationPayload[RewardPayoutNotificationPayload](
			data,
		)
		if err != nil {
			return err
		}
		n.Payload.RewardPayout = reward
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationChildComment:
		id, owner, ts, comment, err := decodeNotificationPayload[ChildCommentNotificationPayload](
			data,
		)
		if err != nil {
			return err
		}
		n.Payload.ChildComment = comment
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationYieldPayout:
		id, owner, ts, yield, err := decodeNotificationPayload[YieldPayoutNotificationPayload](data)
		if err != nil {
			return err
		}
		n.Payload.YieldPayout = yield
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationAutoRedeemed:
		id, owner, ts, redeemed, err := decodeNotificationPayload[AutoRedeemedNotificationPayload](
			data,
		)
		if err != nil {
			return err
		}
		n.Payload.AutoRedeemed = redeemed
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationComboAutoRedeemed:
		id, owner, ts, combo, err := decodeNotificationPayload[ComboAutoRedeemedNotificationPayload](
			data,
		)
		if err != nil {
			return err
		}
		n.Payload.ComboAutoRedeemed = combo
		n.fill(id, owner, ts, ntype)
		return nil
	default:
		return errUnknownNotificationType
	}
}

// fill sets the account-scoped envelope fields shared by every notification
// kind.
func (n *Notification) fill(id int64, owner string, ts int64, ntype NotificationType) {
	n.ID = id
	n.Owner = owner
	n.Timestamp = ts
	n.Type = ntype
}

// decodeNotificationPayload decodes the shared notification envelope: the
// account-scoped id and owner, a timestamp that may arrive as epoch
// milliseconds or an ISO 8601 string, and the kind-specific payload.
func decodeNotificationPayload[T any](
	data []byte,
) (id int64, owner string, ts int64, payload *T, err error) {
	var envelope struct {
		ID        int64              `json:"id"`
		Owner     string             `json:"owner"`
		Timestamp stdjson.RawMessage `json:"timestamp"`
		Payload   T                  `json:"payload"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return 0, "", 0, nil, err
	}
	ts, err = normalizeNotificationTimestamp(envelope.Timestamp)
	if err != nil {
		return 0, "", 0, nil, fmt.Errorf("timestamp: %w", err)
	}
	return envelope.ID, envelope.Owner, ts, &envelope.Payload, nil
}

// normalizeNotificationTimestamp returns epoch milliseconds for either wire form.
// Absent/null/empty timestamps remain zero; invalid timestamps are errors.
func normalizeNotificationTimestamp(value stdjson.RawMessage) (int64, error) {
	instant, err := decodeNotificationDate(value)
	if err != nil {
		return 0, err
	}
	if instant == nil {
		return 0, nil
	}
	return instant.UnixMilli(), nil
}

// DeleteNotificationsParams filters notification deletion requests.
type DeleteNotificationsParams struct {
	IDs []string
}
