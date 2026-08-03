package perps

import (
	"context"
	"encoding/base64"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

const (
	accountNotificationsEndpoint     = "/v1/account/notifications"
	accountNotificationsReadEndpoint = "/v1/account/notifications/read"
)

// PerpsNotificationType identifies an account notification.
type PerpsNotificationType string

const (
	PerpsNotificationPositionOpened     PerpsNotificationType = "position_opened"
	PerpsNotificationPositionIncreased  PerpsNotificationType = "position_increased"
	PerpsNotificationPositionReduced    PerpsNotificationType = "position_reduced"
	PerpsNotificationPositionClosed     PerpsNotificationType = "position_closed"
	PerpsNotificationLimitOrderCanceled PerpsNotificationType = "limit_order_canceled"
	PerpsNotificationLiquidationWarning PerpsNotificationType = "liquidation_warning"
	PerpsNotificationPositionLiquidated PerpsNotificationType = "position_liquidated"
)

// PerpsNotificationOrderType identifies the order that produced a position
// notification.
type PerpsNotificationOrderType string

const (
	PerpsNotificationOrderMarket     PerpsNotificationOrderType = "market"
	PerpsNotificationOrderLimit      PerpsNotificationOrderType = "limit"
	PerpsNotificationOrderTakeProfit PerpsNotificationOrderType = "take_profit"
	PerpsNotificationOrderStopLoss   PerpsNotificationOrderType = "stop_loss"
)

// PerpsMarginType identifies the margin mode of a notification.
type PerpsMarginType string

const (
	PerpsMarginCross    PerpsMarginType = "cross"
	PerpsMarginIsolated PerpsMarginType = "isolated"
)

// ErrUnknownPerpsNotification is returned when a server notification type is
// newer than this SDK. Notification list pages skip such entries so a new
// server type cannot make an otherwise readable page fail.
var ErrUnknownPerpsNotification = errors.New("unknown perps notification type")

// PerpsPositionChangeNotification describes an opened, increased, or reduced
// position.
type PerpsPositionChangeNotification struct {
	ID           string                     `json:"id"`
	Type         PerpsNotificationType      `json:"type"`
	InstrumentID int                        `json:"instrument_id"`
	Side         PerpsSide                  `json:"side"`
	Size         string                     `json:"size"`
	AvgPrice     string                     `json:"avg_price"`
	Leverage     int                        `json:"leverage"`
	OrderType    PerpsNotificationOrderType `json:"order_type,omitempty"`
}

// PerpsPositionClosedNotification describes a position closed by a fill.
type PerpsPositionClosedNotification struct {
	ID           string                     `json:"id"`
	Type         PerpsNotificationType      `json:"type"`
	InstrumentID int                        `json:"instrument_id"`
	Side         PerpsSide                  `json:"side"`
	Size         string                     `json:"size"`
	AvgPrice     string                     `json:"avg_price"`
	PnL          string                     `json:"pnl"`
	OrderType    PerpsNotificationOrderType `json:"order_type,omitempty"`
}

// PerpsLimitOrderCanceledNotification describes a canceled resting order.
type PerpsLimitOrderCanceledNotification struct {
	ID           string                `json:"id"`
	Type         PerpsNotificationType `json:"type"`
	InstrumentID int                   `json:"instrument_id"`
	Side         PerpsSide             `json:"side"`
	Size         string                `json:"size"`
	Price        string                `json:"price"`
}

// PerpsLiquidationWarningNotification describes an isolated or cross-margin
// liquidation warning. Cross-margin warnings have a nil InstrumentID and a
// non-empty AffectedInstruments list.
type PerpsLiquidationWarningNotification struct {
	ID                  string                `json:"id"`
	Type                PerpsNotificationType `json:"type"`
	MarginType          PerpsMarginType       `json:"margin_type"`
	InstrumentID        *int                  `json:"instrument_id"`
	MarkPrice           string                `json:"mark_price"`
	LiquidationPrice    string                `json:"liq_price,omitempty"`
	AffectedInstruments []int                 `json:"affected_instruments,omitempty"`
}

// PerpsPositionLiquidatedNotification describes a partial or full
// liquidation.
type PerpsPositionLiquidatedNotification struct {
	ID           string                `json:"id"`
	Type         PerpsNotificationType `json:"type"`
	InstrumentID int                   `json:"instrument_id"`
	Side         PerpsSide             `json:"side"`
	SizeClosed   string                `json:"size_closed"`
	PnL          *string               `json:"pnl"`
	MarginType   PerpsMarginType       `json:"margin_type"`
	ViaBackstop  bool                  `json:"via_backstop"`
}

// PerpsNotification is a tagged notification. Exactly one variant pointer is
// populated after JSON decoding; Type and ID are also copied for convenient
// dispatch without inspecting the variant.
type PerpsNotification struct {
	ID                 string
	Type               PerpsNotificationType
	PositionChange     *PerpsPositionChangeNotification
	PositionClosed     *PerpsPositionClosedNotification
	LimitOrderCanceled *PerpsLimitOrderCanceledNotification
	LiquidationWarning *PerpsLiquidationWarningNotification
	PositionLiquidated *PerpsPositionLiquidatedNotification
}

// UnmarshalJSON decodes the known notification variants while preserving a
// closed set of typed payloads. Unknown types return ErrUnknownPerpsNotification
// so list-page decoding can omit them without losing known entries.
func (n *PerpsNotification) UnmarshalJSON(data []byte) error {
	var probe struct {
		ID   string                `json:"id"`
		Type PerpsNotificationType `json:"type"`
	}
	if err := stdjson.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("decode perps notification discriminator: %w", err)
	}
	if probe.Type == "" {
		return fmt.Errorf("perps notification type is required")
	}
	*n = PerpsNotification{ID: probe.ID, Type: probe.Type}
	switch probe.Type {
	case PerpsNotificationPositionOpened, PerpsNotificationPositionIncreased, PerpsNotificationPositionReduced:
		var value PerpsPositionChangeNotification
		if err := stdjson.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode perps position change notification: %w", err)
		}
		if err := validatePositionChangeNotification(value); err != nil {
			return err
		}
		n.PositionChange = &value
	case PerpsNotificationPositionClosed:
		var value PerpsPositionClosedNotification
		if err := stdjson.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode perps position closed notification: %w", err)
		}
		if err := validatePositionClosedNotification(value); err != nil {
			return err
		}
		n.PositionClosed = &value
	case PerpsNotificationLimitOrderCanceled:
		var value PerpsLimitOrderCanceledNotification
		if err := stdjson.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode perps canceled-order notification: %w", err)
		}
		if err := validateCanceledOrderNotification(value); err != nil {
			return err
		}
		n.LimitOrderCanceled = &value
	case PerpsNotificationLiquidationWarning:
		var value PerpsLiquidationWarningNotification
		if err := stdjson.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode perps liquidation-warning notification: %w", err)
		}
		if err := validateLiquidationWarningNotification(value); err != nil {
			return err
		}
		n.LiquidationWarning = &value
	case PerpsNotificationPositionLiquidated:
		var value PerpsPositionLiquidatedNotification
		if err := stdjson.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode perps liquidated-position notification: %w", err)
		}
		if err := validatePositionLiquidatedNotification(value); err != nil {
			return err
		}
		n.PositionLiquidated = &value
	default:
		return fmt.Errorf("%w: %q", ErrUnknownPerpsNotification, probe.Type)
	}
	return nil
}

func validateNotificationID(id string) error {
	if id == "" {
		return fmt.Errorf("perps notification id is required")
	}
	return nil
}

func validateNotificationSide(side PerpsSide) error {
	if side != PerpsSideLong && side != PerpsSideShort {
		return fmt.Errorf("perps notification side %q is invalid", side)
	}
	return nil
}

func validateNotificationOrderType(orderType PerpsNotificationOrderType) error {
	if orderType == "" || orderType == PerpsNotificationOrderMarket ||
		orderType == PerpsNotificationOrderLimit ||
		orderType == PerpsNotificationOrderTakeProfit ||
		orderType == PerpsNotificationOrderStopLoss {
		return nil
	}
	return fmt.Errorf("perps notification order type %q is invalid", orderType)
}

func validatePositionChangeNotification(value PerpsPositionChangeNotification) error {
	if err := validateNotificationID(value.ID); err != nil {
		return err
	}
	if value.InstrumentID < 0 || value.Size == "" || value.AvgPrice == "" || value.Leverage <= 0 {
		return fmt.Errorf("perps position change notification has invalid required fields")
	}
	if err := validateNotificationSide(value.Side); err != nil {
		return err
	}
	return validateNotificationOrderType(value.OrderType)
}

func validatePositionClosedNotification(value PerpsPositionClosedNotification) error {
	if err := validateNotificationID(value.ID); err != nil {
		return err
	}
	if value.InstrumentID < 0 || value.Size == "" || value.AvgPrice == "" || value.PnL == "" {
		return fmt.Errorf("perps position closed notification has invalid required fields")
	}
	if err := validateNotificationSide(value.Side); err != nil {
		return err
	}
	return validateNotificationOrderType(value.OrderType)
}

func validateCanceledOrderNotification(value PerpsLimitOrderCanceledNotification) error {
	if err := validateNotificationID(value.ID); err != nil {
		return err
	}
	if value.InstrumentID < 0 || value.Size == "" || value.Price == "" {
		return fmt.Errorf("perps canceled-order notification has invalid required fields")
	}
	return validateNotificationSide(value.Side)
}

func validateLiquidationWarningNotification(value PerpsLiquidationWarningNotification) error {
	if err := validateNotificationID(value.ID); err != nil {
		return err
	}
	if value.MarkPrice == "" {
		return fmt.Errorf("perps liquidation warning mark price is required")
	}
	switch value.MarginType {
	case PerpsMarginIsolated:
		if value.InstrumentID == nil || *value.InstrumentID < 0 || value.LiquidationPrice == "" {
			return fmt.Errorf("perps isolated liquidation warning has invalid required fields")
		}
	case PerpsMarginCross:
		if value.InstrumentID != nil || value.LiquidationPrice != "" {
			return fmt.Errorf("perps cross liquidation warning has invalid instrument or liquidation price")
		}
	default:
		return fmt.Errorf("perps liquidation warning margin type %q is invalid", value.MarginType)
	}
	return nil
}

func validatePositionLiquidatedNotification(value PerpsPositionLiquidatedNotification) error {
	if err := validateNotificationID(value.ID); err != nil {
		return err
	}
	if value.InstrumentID < 0 || value.SizeClosed == "" {
		return fmt.Errorf("perps liquidated-position notification has invalid required fields")
	}
	if err := validateNotificationSide(value.Side); err != nil {
		return err
	}
	if value.MarginType != PerpsMarginCross && value.MarginType != PerpsMarginIsolated {
		return fmt.Errorf("perps liquidated-position margin type %q is invalid", value.MarginType)
	}
	return nil
}

// PerpsNotificationEntry is one notification with account read state.
type PerpsNotificationEntry struct {
	Notification PerpsNotification `json:"notification"`
	ReadAt       *int64            `json:"read_at"`
	Timestamp    int64             `json:"ts"`
}

// PerpsNotificationsPage is one page of account notifications.
type PerpsNotificationsPage struct {
	Items                 []PerpsNotificationEntry `json:"items"`
	Unread                int                      `json:"unread"`
	DurableSourceSequence int64                    `json:"durable_source_seq"`
	More                  bool                     `json:"has_more"`
	NextCursor            string                   `json:"next_cursor"`
}

// UnmarshalJSON skips notification entries whose type is unknown to this SDK,
// matching the official clients' forward-compatible list behavior.
func (p *PerpsNotificationsPage) UnmarshalJSON(data []byte) error {
	var wire struct {
		Items                 []stdjson.RawMessage `json:"items"`
		Unread                int                  `json:"unread"`
		DurableSourceSequence int64                `json:"durable_source_seq"`
		More                  bool                 `json:"has_more"`
		NextCursor            *string              `json:"next_cursor"`
	}
	if err := stdjson.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode perps notifications page: %w", err)
	}
	items := make([]PerpsNotificationEntry, 0, len(wire.Items))
	for i, raw := range wire.Items {
		var entry PerpsNotificationEntry
		if err := stdjson.Unmarshal(raw, &entry); err != nil {
			if errors.Is(err, ErrUnknownPerpsNotification) {
				continue
			}
			return fmt.Errorf("decode perps notification item %d: %w", i, err)
		}
		items = append(items, entry)
	}
	*p = PerpsNotificationsPage{
		Items:                 items,
		Unread:                wire.Unread,
		DurableSourceSequence: wire.DurableSourceSequence,
		More:                  wire.More && wire.NextCursor != nil && *wire.NextCursor != "",
	}
	if wire.NextCursor != nil {
		p.NextCursor = *wire.NextCursor
	}
	return nil
}

// NotificationsParams filters one notification page.
type NotificationsParams struct {
	SinceSequence int64
	Limit         int
	Cursor        string
}

// PerpsNotificationsParams is an explicit package-prefixed alias.
type PerpsNotificationsParams = NotificationsParams

// GetNotificationsPage returns one page of account notifications. The raw
// server cursor is available as NextCursor for the next request.
func (c *AuthenticatedClient) GetNotificationsPage(
	ctx context.Context,
	p NotificationsParams,
) (PerpsNotificationsPage, error) {
	if p.SinceSequence < 0 {
		return PerpsNotificationsPage{}, fmt.Errorf("perps: notification since sequence must be non-negative")
	}
	if p.Limit < 0 {
		return PerpsNotificationsPage{}, fmt.Errorf("perps: notification limit must be non-negative")
	}
	query := url.Values{}
	if p.SinceSequence != 0 {
		query.Set("since_seq", strconv.FormatInt(p.SinceSequence, 10))
	}
	if p.Limit != 0 {
		query.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		query.Set("cursor", p.Cursor)
	}
	var out PerpsNotificationsPage
	if err := c.getAuthenticatedJSON(ctx, accountNotificationsEndpoint, query, &out); err != nil {
		return PerpsNotificationsPage{}, err
	}
	return out, nil
}

// GetUnreadNotificationsCount returns the account's current unread count.
func (c *AuthenticatedClient) GetUnreadNotificationsCount(ctx context.Context) (int, error) {
	var out struct {
		Unread int `json:"unread"`
	}
	query := url.Values{"limit": {"1"}}
	if err := c.getAuthenticatedJSON(ctx, accountNotificationsEndpoint, query, &out); err != nil {
		return 0, err
	}
	return out.Unread, nil
}

// PerpsNotificationReadCursor identifies the inclusive boundary for marking
// notifications read.
type PerpsNotificationReadCursor struct {
	ID        string
	Timestamp int64
}

// MarkNotificationsReadParams selects notification IDs or an inclusive
// timestamp/ID boundary. Exactly one selector must be provided.
type MarkNotificationsReadParams struct {
	IDs    []string
	Before *PerpsNotificationReadCursor
}

// PerpsNotificationsReadParams is an explicit package-prefixed alias.
type PerpsNotificationsReadParams = MarkNotificationsReadParams

// MarkNotificationsRead marks selected account notifications as read.
func (c *AuthenticatedClient) MarkNotificationsRead(
	ctx context.Context,
	p MarkNotificationsReadParams,
) error {
	hasIDs := len(p.IDs) > 0
	if hasIDs == (p.Before != nil) {
		return fmt.Errorf("perps: provide exactly one of notification IDs or Before")
	}
	body := any(nil)
	if hasIDs {
		ids := make([]string, len(p.IDs))
		copy(ids, p.IDs)
		for _, id := range ids {
			if id == "" {
				return fmt.Errorf("perps: notification IDs must not be empty")
			}
		}
		body = struct {
			IDs []string `json:"ids"`
		}{IDs: ids}
	} else {
		if p.Before.ID == "" || p.Before.Timestamp < 0 {
			return fmt.Errorf("perps: notification Before cursor is invalid")
		}
		cursor, err := encodeNotificationReadCursor(*p.Before)
		if err != nil {
			return err
		}
		body = struct {
			Before string `json:"before"`
		}{Before: cursor}
	}

	var response struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := c.postAuthenticatedJSON(ctx, accountNotificationsReadEndpoint, body, &response); err != nil {
		return err
	}
	if response.Status == "ok" {
		return nil
	}
	if response.Status == "err" && response.Error != "" {
		return fmt.Errorf("perps: mark notifications read: %s", response.Error)
	}
	return fmt.Errorf("perps: unexpected mark notifications read response status %q", response.Status)
}

func encodeNotificationReadCursor(cursor PerpsNotificationReadCursor) (string, error) {
	payload, err := stdjson.Marshal(struct {
		Timestamp int64  `json:"ts"`
		ID        string `json:"id"`
	}{Timestamp: cursor.Timestamp, ID: cursor.ID})
	if err != nil {
		return "", fmt.Errorf("perps: encode notification read cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
