package data

import (
	"context"
	"iter"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"
)

func (c *Client) GetPositions(ctx context.Context, p PositionsParams) (Page[Position], error) {
	q := url.Values{}
	if err := optionalUser(q, p.User); err != nil {
		return Page[Position]{}, err
	}
	if err := selectors(q, p.Selectors, marketConditions); err != nil {
		return Page[Position]{}, err
	}
	if err := window(q, p.Window, false); err != nil {
		return Page[Position]{}, err
	}
	if err := enum(q, "status", p.Status, "OPEN", "REDEEMABLE", "REDEEMABLE_LOST", "MERGEABLE", "CLOSED"); err != nil {
		return Page[Position]{}, err
	}
	if err := enum(q, "filter_type", p.FilterType, "CASH", "TOKENS"); err != nil {
		return Page[Position]{}, err
	}
	if err := amountParam(q, "filter_amount", p.FilterAmount); err != nil {
		return Page[Position]{}, err
	}
	if err := enum(q, "sort_by", p.SortBy, "CURRENT_VALUE", "PRICE", "TOKENS", "UNREALIZED_PNL", "REALIZED_PNL", "TOTAL_PNL", "TIMESTAMP"); err != nil {
		return Page[Position]{}, err
	}
	if err := direction(q, p.SortDirection); err != nil {
		return Page[Position]{}, err
	}
	if p.User == "" &&
		(q.Get("condition_id") == "" || strings.Contains(q.Get("condition_id"), ",")) {
		return Page[Position]{}, input("selectors", "provide a user or exactly one condition ID")
	}
	if p.User == "" &&
		(p.Selectors.EventIDs != nil || p.Status == PositionStatusRedeemableLost || p.Status == PositionStatusMergeable) {
		return Page[Position]{}, input(
			"user",
			"is required for event, lost-redeemable, and mergeable filters",
		)
	}
	if p.Status == PositionStatusClosed && p.IncludeArchived != nil && *p.IncludeArchived {
		return Page[Position]{}, input("include_archived", "cannot be combined with CLOSED")
	}
	if strings.TrimSpace(p.Title) != "" {
		if utf8.RuneCountInString(p.Title) > 200 {
			return Page[Position]{}, input("title", "must contain at most 200 characters")
		}
		q.Set("title", p.Title)
	}
	boolParam(q, "include_archived", p.IncludeArchived)
	return getPage[Position](ctx, c, "/v2/positions", q, p.Page, 1000)
}

func (c *Client) IterPositions(ctx context.Context, p PositionsParams) iter.Seq2[Position, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[Position], error) {
		request := p
		request.Page = page
		return c.GetPositions(ctx, request)
	})
}

func (c *Client) GetComboPositions(
	ctx context.Context,
	p ComboPositionsParams,
) (Page[ComboPosition], error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return Page[ComboPosition]{}, err
	}
	if err := conditionIDs(q, p.ConditionIDs, comboConditions); err != nil {
		return Page[ComboPosition]{}, err
	}
	if err := enum(q, "sort_by", p.SortBy, "FIRST_ENTRY", "ENTRY_COST", "CURRENT_VALUE", "UPDATED"); err != nil {
		return Page[ComboPosition]{}, err
	}
	if err := direction(q, p.SortDirection); err != nil {
		return Page[ComboPosition]{}, err
	}
	if err := timestampParam(q, "updated_after", p.UpdatedAfter, true); err != nil {
		return Page[ComboPosition]{}, err
	}
	if err := timestampParam(q, "updated_before", p.UpdatedBefore, true); err != nil {
		return Page[ComboPosition]{}, err
	}
	if p.UpdatedAfter != nil && p.UpdatedBefore != nil &&
		p.UpdatedBefore.Unix() < p.UpdatedAfter.Unix() {
		return Page[ComboPosition]{}, input("updated_before", "must be at least updated_after")
	}
	if p.Statuses != nil {
		if len(p.Statuses) == 0 {
			return Page[ComboPosition]{}, input("status", "must be nonempty")
		}
		statuses := make([]string, 0, len(p.Statuses))
		for _, status := range p.Statuses {
			if status == "" {
				return Page[ComboPosition]{}, input("status", "must contain valid combo statuses")
			}
			if err := enum(q, "status", status, "OPEN", "REDEEMABLE", "PARTIAL", "RESOLVED_PARTIAL", "RESOLVED_WIN", "RESOLVED_LOSS"); err != nil {
				return Page[ComboPosition]{}, err
			}
			if !slices.Contains(statuses, string(status)) {
				statuses = append(statuses, string(status))
			}
		}
		if slices.Contains(statuses, "REDEEMABLE") && len(statuses) != 1 {
			return Page[ComboPosition]{}, input("status", "REDEEMABLE must be the only status")
		}
		q.Set("status", strings.Join(statuses, ","))
	}
	return getPage[ComboPosition](ctx, c, "/v2/positions/combos", q, p.Page, 1000)
}

func (c *Client) IterComboPositions(
	ctx context.Context,
	p ComboPositionsParams,
) iter.Seq2[ComboPosition, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[ComboPosition], error) {
		request := p
		request.Page = page
		return c.GetComboPositions(ctx, request)
	})
}
