package clob

import (
	"errors"
	"fmt"
	"time"

	stdjson "encoding/json"

	json "github.com/go-json-experiment/json"
)

// Credentials are the Polymarket API credentials used for authenticated CLOB requests.
type Credentials struct {
	Key        string `json:"key"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

type apiKeyRaw struct {
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

type builderAPIKeyRaw struct {
	Key        string `json:"key"`
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

func (r builderAPIKeyRaw) credentials() Credentials {
	key := r.Key
	if key == "" {
		key = r.APIKey
	}
	return Credentials{
		Key:        key,
		Secret:     r.Secret,
		Passphrase: r.Passphrase,
	}
}

// WSAuth contains the raw credentials for authenticated CLOB websocket
// subscriptions. Unlike HTTP L2 auth, the websocket user channel does not
// use an HMAC timestamp/signature envelope.
type WSAuth struct {
	Key        string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp,omitempty"`
	Signature  string `json:"signature,omitempty"`
}

// APIKeysResponse is the response payload for listing API keys.
type APIKeysResponse struct {
	APIKeys []string `json:"apiKeys"`
}

// BanStatus reports whether the account is currently restricted to closed-only mode.
type BanStatus struct {
	ClosedOnly bool `json:"closed_only"`
}

// ReadonlyAPIKeyResponse is the response from creating a readonly API key.
type ReadonlyAPIKeyResponse struct {
	APIKey string `json:"apiKey"`
}

// ReadonlyAPIKeysResponse is the response from listing readonly API keys.
type ReadonlyAPIKeysResponse struct {
	ReadonlyAPIKeys []string `json:"readonly_api_keys"`
}

// DeleteReadonlyAPIKeyRequest is the request payload for removing a readonly API key.
type DeleteReadonlyAPIKeyRequest struct {
	Key string `json:"key"`
}

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
	NotificationOrderCancellation  NotificationType = 1
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
	OrderCancellation  *OrderNotificationPayload
	OrderFill          *OrderNotificationPayload
	MarketRegistered   *MarketNotificationPayload
	MarketResolved     *MarketNotificationPayload
	RewardPayout       *RewardPayoutNotificationPayload
	ChildComment       *ChildCommentNotificationPayload
	YieldPayout        *YieldPayoutNotificationPayload
	OrderFillFailed    *OrderNotificationPayload
	AutoRedeemed       *AutoRedeemedNotificationPayload
	ComboAutoRedeemed  *ComboAutoRedeemedNotificationPayload
}

// OrderNotificationPayload is the payload of an order lifecycle notification
// (cancellation, fill, and failed fill share this shape).
type OrderNotificationPayload struct {
	AssetID         string `json:"asset_id"`
	ConditionID     string `json:"market"`
	OrderID         string `json:"order_id"`
	Side            Side   `json:"side"`
	OrderType       string `json:"type,omitempty"`
	Price           string `json:"price"`
	OriginalSize    string `json:"original_size"`
	MatchedSize     string `json:"matched_size"`
	RemainingSize   string `json:"remaining_size"`
	Outcome         string `json:"outcome"`
	OutcomeIndex    int64  `json:"outcome_index"`
	TransactionHash string `json:"transaction_hash,omitempty"`
	TradeID         string `json:"trade_id,omitempty"`
	Question        string `json:"question,omitempty"`
	MarketSlug      string `json:"market_slug,omitempty"`
	Icon            string `json:"icon,omitempty"`
	Image           string `json:"image,omitempty"`
	EventSlug       string `json:"eventSlug,omitempty"`
	SeriesSlug      string `json:"seriesSlug,omitempty"`
}

// MarketNotificationToken is one outcome token inside a market lifecycle
// notification payload. On a market-resolved notification, Winner marks the
// winning outcome.
type MarketNotificationToken struct {
	TokenID  string `json:"token_id"`
	Outcome  string `json:"outcome"`
	Price    string `json:"price,omitempty"`
	Winner   bool   `json:"winner"`
}

// MarketNotificationRewardsRate is one per-asset daily reward rate carried on
// a market lifecycle notification.
type MarketNotificationRewardsRate struct {
	AssetAddress string  `json:"asset_address"`
	DailyRate    float64 `json:"rewards_daily_rate"`
}

// MarketNotificationRewards carries liquidity-rewards parameters on a market
// lifecycle notification.
type MarketNotificationRewards struct {
	MinSize  float64                          `json:"min_size"`
	MaxSpread float64                          `json:"max_spread"`
	Rates    []MarketNotificationRewardsRate  `json:"rates,omitempty"`
}

// MarketNotificationPayload is the payload of a market lifecycle notification
// (market registered and market resolved share this shape).
type MarketNotificationPayload struct {
	ConditionID              string                          `json:"condition_id"`
	QuestionID               string                          `json:"question_id"`
	Question                 string                          `json:"question"`
	Description              string                          `json:"description"`
	MarketSlug               string                          `json:"market_slug"`
	Icon                     string                          `json:"icon"`
	Image                    string                          `json:"image"`
	Fpmm                     string                          `json:"fpmm"`
	Active                   bool                            `json:"active"`
	Closed                   bool                            `json:"closed"`
	Archived                *bool                           `json:"archived,omitempty"`
	AcceptingOrders          bool                            `json:"accepting_orders"`
	AcceptingOrdersTimestamp *int64                          `json:"accepting_order_timestamp,omitempty"`
	EnableOrderBook         *bool                           `json:"enable_order_book,omitempty"`
	EndDate                 *int64                          `json:"end_date_iso,omitempty"`
	GameStartTime           *int64                          `json:"game_start_time,omitempty"`
	SecondsDelay             int                             `json:"seconds_delay"`
	MinimumOrderSize         string                          `json:"minimum_order_size"`
	MinimumTickSize         string                          `json:"minimum_tick_size"`
	MakerBaseFee            *int                            `json:"maker_base_fee,omitempty"`
	TakerBaseFee            *int                            `json:"taker_base_fee,omitempty"`
	NotificationsEnabled    *bool                           `json:"notifications_enabled,omitempty"`
	NegRisk                 *bool                           `json:"neg_risk,omitempty"`
	NegRiskMarketID         string                          `json:"neg_risk_market_id,omitempty"`
	NegRiskRequestID        string                          `json:"neg_risk_request_id,omitempty"`
	Is5050Outcome           *bool                           `json:"is_50_50_outcome,omitempty"`
	Rewards                 *MarketNotificationRewards      `json:"rewards,omitempty"`
	Tokens                  []MarketNotificationToken       `json:"tokens"`
	Tags                    []string                        `json:"tags,omitempty"`
	EventSlug               string                          `json:"eventSlug,omitempty"`
}

// RewardPayoutNotificationPayload is the payload of a liquidity-reward
// payout notification.
type RewardPayoutNotificationPayload struct {
	ProxyWallet    string `json:"proxyWallet"`
	Reward         string `json:"reward"`
	TransactionHash string `json:"txnHash"`
}

// YieldPayoutNotificationPayload is the payload of a yield payout notification.
type YieldPayoutNotificationPayload struct {
	ProxyWallet    string `json:"proxyWallet"`
	Amount         string `json:"amount"`
	TransactionHash string `json:"txnHash"`
}

// ChildCommentNotificationPayload is the payload of a child-comment
// notification: the reply comment, its author's profile, and the event or
// series the thread belongs to.
type ChildCommentNotificationPayload struct {
	ID                int64   `json:"id"`
	Body              *string `json:"body,omitempty"`
	ParentEntityType  *string `json:"parentEntityType,omitempty"`
	ParentEntityID    *int64  `json:"parentEntityID,omitempty"`
	ParentCommentID   *int64  `json:"parentCommentID,omitempty"`
	UserAddress       *string `json:"userAddress,omitempty"`
	CreatedAt         *int64  `json:"createdAt,omitempty"`
	EventSlug         *string `json:"eventSlug,omitempty"`
	EventTitle        *string `json:"eventTitle,omitempty"`
	SeriesSlug        *string `json:"seriesSlug,omitempty"`
	SeriesTitle       *string `json:"seriesTitle,omitempty"`
	Image             *string `json:"image,omitempty"`
}

// AutoRedeemedNotificationPayload is the payload of an auto-redeem
// notification: a winning position redeemed on-chain on the account's behalf.
type AutoRedeemedNotificationPayload struct {
	ProxyWallet      string `json:"proxyWallet"`
	Amount           string `json:"amount"`
	ConditionID      string `json:"conditionId"`
	Question         string `json:"question"`
	Image            string `json:"image"`
	MarketSlug       string `json:"slug"`
	Position         *string `json:"position,omitempty"`
	MarketURL        *string `json:"marketUrl,omitempty"`
	PortfolioURL     *string `json:"portfolioUrl,omitempty"`
	NegRisk          bool   `json:"negRisk"`
	TransactionHash  string `json:"txnHash"`
}

// ComboAutoRedeemedNotificationPayload is the payload of a combo auto-redeem
// notification: a winning combo position redeemed on-chain on the account's
// behalf. Legs is the combo arity.
type ComboAutoRedeemedNotificationPayload struct {
	ProxyWallet     string `json:"proxyWallet"`
	Amount          string `json:"amount"`
	PositionID      string `json:"positionId"`
	ConditionID     string `json:"conditionId"`
	OutcomeIndex    int    `json:"outcomeIndex"`
	Legs            int    `json:"legs"`
	PortfolioURL    *string `json:"portfolioUrl,omitempty"`
	TransactionHash string `json:"txnHash"`
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
// Empty strings are normalized to nil for optional fields.
func (n *Notification) UnmarshalJSON(data []byte) error {
	typeVal, ok, err := probeNotificationType(data)
	if err != nil {
		return fmt.Errorf("probe notification type: %w", err)
	}
	if !ok {
		return fmt.Errorf("notification missing type field")
	}

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
		id, owner, ts, reward, err := decodeNotificationPayload[RewardPayoutNotificationPayload](data)
		if err != nil {
			return err
		}
		n.Payload.RewardPayout = reward
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationChildComment:
		id, owner, ts, comment, err := decodeNotificationPayload[ChildCommentNotificationPayload](data)
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
		id, owner, ts, redeemed, err := decodeNotificationPayload[AutoRedeemedNotificationPayload](data)
		if err != nil {
			return err
		}
		n.Payload.AutoRedeemed = redeemed
		n.fill(id, owner, ts, ntype)
		return nil
	case NotificationComboAutoRedeemed:
		id, owner, ts, combo, err := decodeNotificationPayload[ComboAutoRedeemedNotificationPayload](data)
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
func decodeNotificationPayload[T any](data []byte) (id int64, owner string, ts int64, payload *T, err error) {
	var envelope struct {
		ID        int64          `json:"id"`
		Owner     string         `json:"owner"`
		Timestamp stdjson.Number `json:"timestamp"`
		Payload   T              `json:"payload"`
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

// normalizeNotificationTimestamp accepts epoch milliseconds as a number or
// numeric string, or an ISO 8601 date string. It returns the epoch
// milliseconds. An empty or invalid value is treated as zero.
func normalizeNotificationTimestamp(value stdjson.Number) (int64, error) {
	if value == "" {
		return 0, nil
	}
	// Try as an integer first.
	if i, err := value.Int64(); err == nil {
		return i, nil
	}
	// Try as a float (epoch ms with fractional part).
	if f, err := value.Float64(); err == nil {
		return int64(f), nil
	}
	// Try as an ISO 8601 date string.
	s := string(value)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli(), nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UnixMilli(), nil
	}
	return 0, fmt.Errorf("unrecognized timestamp %q", s)
}

// DeleteNotificationsParams filters notification deletion requests.
type DeleteNotificationsParams struct {
	IDs []string
}

// AssetType identifies the Polymarket asset namespace used in allowance requests.
type AssetType string

const (
	// AssetTypeCollateral is the USDC collateral asset namespace.
	AssetTypeCollateral AssetType = "COLLATERAL"
	// AssetTypeConditional is the conditional token asset namespace.
	AssetTypeConditional AssetType = "CONDITIONAL"
)

// BalanceAllowanceParams configures a balance or allowance lookup.
type BalanceAllowanceParams struct {
	AssetType     AssetType
	TokenID       string
	SignatureType *SignatureType
}

// BalanceAllowanceResponse reports the current balance and spender allowances.
type BalanceAllowanceResponse struct {
	Balance    string            `json:"balance"`
	Allowances map[string]string `json:"allowances"`
}

// OrderScoringParams filters a single-order scoring lookup.
type OrderScoringParams struct {
	OrderID string
}

// OrderScoringResponse reports whether an order is scoring for rewards.
type OrderScoringResponse struct {
	Scoring bool `json:"scoring"`
}

// OrdersScoringParams configures a batch scoring lookup.
type OrdersScoringParams struct {
	OrderIDs []string
}

// OrdersScoringResponse maps order IDs to their scoring status.
type OrdersScoringResponse map[string]bool

// CancelMarketOrdersRequest scopes cancelation to a market and/or asset.
type CancelMarketOrdersRequest struct {
	Market  string `json:"market,omitzero"`
	AssetID string `json:"asset_id,omitzero"`
}
