package data

import (
	"context"
	"iter"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) GetHolders(ctx context.Context, p HoldersParams) (Page[MetaHolder], error) {
	q := url.Values{}
	if err := conditionIDs(q, p.ConditionIDs, marketConditions); err != nil {
		return Page[MetaHolder]{}, err
	}
	if q.Get("condition_id") == "" {
		return Page[MetaHolder]{}, input("condition_id", "is required")
	}
	if err := amountParam(q, "min_balance", p.MinBalance); err != nil {
		return Page[MetaHolder]{}, err
	}
	maximum := 1000
	if p.IncludePnL != nil && *p.IncludePnL {
		maximum = 100
		if strings.Contains(q.Get("condition_id"), ",") {
			return Page[MetaHolder]{}, input("include_pnl", "requires exactly one condition")
		}
	}
	boolParam(q, "include_pnl", p.IncludePnL)
	return getPage[MetaHolder](ctx, c, "/v2/holders", q, p.Page, maximum)
}

func (c *Client) IterHolders(ctx context.Context, p HoldersParams) iter.Seq2[MetaHolder, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[MetaHolder], error) {
		request := p
		request.Page = page
		return c.GetHolders(ctx, request)
	})
}

func (c *Client) GetOpenInterest(ctx context.Context, conditions []string) ([]OpenInterest, error) {
	q := url.Values{}
	if err := conditionIDs(q, conditions, marketConditions); err != nil {
		return nil, err
	}
	return getList[OpenInterest](ctx, c, "/v2/oi", q)
}

func (c *Client) GetLiveVolume(ctx context.Context, events []int32) (*LiveVolume, error) {
	q := url.Values{}
	if err := eventIDs(q, events); err != nil {
		return nil, err
	}
	if q.Get("event_id") == "" {
		return nil, input("event_id", "is required")
	}
	return getValue[LiveVolume](ctx, c, "/v2/live-volume", q, false)
}

func (c *Client) GetPriceHistory(
	ctx context.Context,
	p PriceHistoryParams,
) (Page[PriceHistoryPoint], error) {
	q := url.Values{}
	if p.AssetID == "" {
		return Page[PriceHistoryPoint]{}, input("asset_id", "is required")
	}
	q.Set("token_id", p.AssetID)
	choices := 0
	if p.Interval != "" {
		choices++
	}
	if p.Start != nil {
		choices++
	}
	if p.AsOf != nil {
		choices++
	}
	if choices != 1 {
		return Page[PriceHistoryPoint]{}, input(
			"price_history",
			"choose exactly one of interval, start, or as_of",
		)
	}
	if p.End != nil && p.Start == nil {
		return Page[PriceHistoryPoint]{}, input("end", "requires start")
	}
	if err := enum(q, "interval", p.Interval, "max", "all", "1m", "1w", "1d", "6h", "1h"); err != nil {
		return Page[PriceHistoryPoint]{}, err
	}
	if err := timestampParam(q, "start", p.Start, false); err != nil {
		return Page[PriceHistoryPoint]{}, err
	}
	if err := timestampParam(q, "end", p.End, false); err != nil {
		return Page[PriceHistoryPoint]{}, err
	}
	if err := timestampParam(q, "as_of", p.AsOf, false); err != nil {
		return Page[PriceHistoryPoint]{}, err
	}
	if p.Start != nil {
		end := time.Now().Unix()
		if p.End != nil {
			end = p.End.Unix()
		}
		if end < p.Start.Unix() || end-p.Start.Unix() > 15*86400 {
			return Page[PriceHistoryPoint]{}, input(
				"window",
				"must be ordered and span at most 15 days",
			)
		}
	}
	if p.AsOf != nil && (p.BucketSeconds != 0 || p.Page.Limit != 0) {
		return Page[PriceHistoryPoint]{}, input("as_of", "forbids bucket_seconds and page.limit")
	}
	if p.BucketSeconds != 0 {
		minimum := 60
		switch p.Interval {
		case PriceHistoryMax, PriceHistoryAll, PriceHistoryMonth:
			minimum = 600
		case PriceHistoryWeek:
			minimum = 300
		}
		if p.BucketSeconds < minimum || p.BucketSeconds > 86400 {
			return Page[PriceHistoryPoint]{}, input(
				"bucket_seconds",
				"is outside the interval's supported range",
			)
		}
		q.Set("bucket_seconds", strconv.Itoa(p.BucketSeconds))
	}
	return getPage[PriceHistoryPoint](ctx, c, "/v2/prices-history", q, p.Page, 10000)
}

func (c *Client) IterPriceHistory(
	ctx context.Context,
	p PriceHistoryParams,
) iter.Seq2[PriceHistoryPoint, error] {
	// Resolve an implicit window end once, so every page is bound to the same
	// query even if the traversal crosses a second boundary.
	if p.Start != nil && p.End == nil {
		end := time.Now().UTC()
		p.End = &end
	}
	return walk(ctx, p.Page, func(page PageParams) (Page[PriceHistoryPoint], error) {
		request := p
		request.Page = page
		return c.GetPriceHistory(ctx, request)
	})
}

func (c *Client) GetResolutions(ctx context.Context, p ResolutionsParams) ([]Resolution, error) {
	q := url.Values{}
	choices := 0
	if p.QuestionID != "" {
		choices++
		q.Set("question_id", p.QuestionID)
	}
	if p.ConditionIDs != nil {
		choices++
	}
	if p.EventIDs != nil {
		choices++
	}
	if choices != 1 {
		return nil, input(
			"resolutions",
			"choose exactly one of question ID, condition IDs, or event IDs",
		)
	}
	if err := conditionIDs(q, p.ConditionIDs, marketConditions); err != nil {
		return nil, err
	}
	if err := eventIDs(q, p.EventIDs); err != nil {
		return nil, err
	}
	if p.EventIDs != nil && strings.Count(q.Get("event_id"), ",") >= 20 {
		return nil, input("event_id", "accepts at most 20 distinct identifiers")
	}
	return getList[Resolution](ctx, c, "/v2/resolutions", q)
}
