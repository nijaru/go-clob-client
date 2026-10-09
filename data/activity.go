package data

import (
	"context"
	"iter"
	"net/url"
	"strings"
)

func (c *Client) GetTrades(ctx context.Context, p TradesParams) (Page[Trade], error) {
	q := url.Values{}
	if err := optionalUser(q, p.User); err != nil {
		return Page[Trade]{}, err
	}
	if err := selectors(q, p.Selectors, feedConditions); err != nil {
		return Page[Trade]{}, err
	}
	if err := window(q, p.Window, true); err != nil {
		return Page[Trade]{}, err
	}
	if err := enum(q, "side", p.Side, "BUY", "SELL"); err != nil {
		return Page[Trade]{}, err
	}
	if err := enum(q, "filter_type", p.FilterType, "CASH", "TOKENS"); err != nil {
		return Page[Trade]{}, err
	}
	if err := amountParam(q, "filter_amount", p.FilterAmount); err != nil {
		return Page[Trade]{}, err
	}
	boolParam(q, "taker_only", p.TakerOnly)
	return getPage[Trade](ctx, c, "/v2/trades", q, p.Page, 1000)
}

func (c *Client) IterTrades(ctx context.Context, p TradesParams) iter.Seq2[Trade, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[Trade], error) {
		request := p
		request.Page = page
		return c.GetTrades(ctx, request)
	})
}

func (c *Client) GetActivity(ctx context.Context, p ActivityParams) (Page[Activity], error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return Page[Activity]{}, err
	}
	if err := selectors(q, p.Selectors, feedConditions); err != nil {
		return Page[Activity]{}, err
	}
	if err := window(q, p.Window, true); err != nil {
		return Page[Activity]{}, err
	}
	if err := enum(q, "side", p.Side, "BUY", "SELL"); err != nil {
		return Page[Activity]{}, err
	}
	if err := direction(q, p.SortDirection); err != nil {
		return Page[Activity]{}, err
	}
	if p.Types != nil {
		if len(p.Types) == 0 {
			return Page[Activity]{}, input("type", "must be nonempty")
		}
		types := make([]string, len(p.Types))
		for i, kind := range p.Types {
			if err := enum(q, "type", kind, "TRADE", "SPLIT", "MERGE", "REDEEM", "REWARD", "CONVERSION", "MIGRATION", "DEPOSIT", "WITHDRAWAL", "YIELD", "MAKER_REBATE", "TAKER_REBATE", "REFERRAL_REWARD", "TIP"); err != nil {
				return Page[Activity]{}, err
			}
			if kind == "" {
				return Page[Activity]{}, input("type", "must contain valid activity types")
			}
			types[i] = string(kind)
		}
		q.Set("type", strings.Join(types, ","))
	}
	// Deposits and withdrawals are part of the public v2 activity union.
	q.Set("exclude_deposits_withdrawals", "false")
	return getPage[Activity](ctx, c, "/v2/activity", q, p.Page, 1000)
}

func (c *Client) IterActivity(ctx context.Context, p ActivityParams) iter.Seq2[Activity, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[Activity], error) {
		request := p
		request.Page = page
		return c.GetActivity(ctx, request)
	})
}

func (c *Client) GetComboActivity(
	ctx context.Context,
	p ComboActivityParams,
) (Page[ComboActivity], error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return Page[ComboActivity]{}, err
	}
	if err := conditionIDs(q, p.ConditionIDs, comboConditions); err != nil {
		return Page[ComboActivity]{}, err
	}
	return getPage[ComboActivity](ctx, c, "/v2/activity/combos", q, p.Page, 1000)
}

func (c *Client) IterComboActivity(
	ctx context.Context,
	p ComboActivityParams,
) iter.Seq2[ComboActivity, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[ComboActivity], error) {
		request := p
		request.Page = page
		return c.GetComboActivity(ctx, request)
	})
}
