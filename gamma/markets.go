package gamma

import (
	"context"
	"iter"
	"net/url"
	"strconv"
	"strings"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func marketOptionsQuery(options []MarketOptions) url.Values {
	if len(options) == 0 {
		return nil
	}
	query := url.Values{}
	setBool(query, "include_tag", options[0].IncludeTag)
	setString(query, "locale", options[0].Locale)
	return query
}

// GetMarket returns a single market by its ID. Optional Rust-compatible
// include_tag behavior can be supplied as the third argument.
func (c *Client) GetMarket(
	ctx context.Context,
	id string,
	options ...MarketOptions,
) (*Market, error) {
	var out Market
	query := marketOptionsQuery(options)
	err := c.http.GetJSON(
		ctx,
		marketsEndpoint+"/"+url.PathEscape(id),
		query,
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMarketBySlug returns a single market by its slug. Optional Rust-compatible
// include_tag behavior can be supplied as the third argument.
func (c *Client) GetMarketBySlug(
	ctx context.Context,
	slug string,
	options ...MarketOptions,
) (*Market, error) {
	var out Market
	query := marketOptionsQuery(options)
	err := c.http.GetJSON(
		ctx,
		marketsEndpoint+"/slug/"+url.PathEscape(slug),
		query,
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMarkets returns a list of markets based on the provided filters.
func (c *Client) GetMarkets(ctx context.Context, params MarketFilterParams) ([]Market, error) {
	if err := validateOffset(params.Limit, params.Offset, -1); err != nil {
		return nil, err
	}
	var out []Market
	err := c.http.GetJSON(ctx, marketsEndpoint, marketQuery(params), polyhttp.AuthNone, &out)
	return out, err
}

func marketQuery(params MarketFilterParams) url.Values {
	query := url.Values{}
	setString(query, "locale", params.Locale)
	setBool(query, "decimalized", params.Decimalized)
	setBool(query, "rfq_enabled", params.RfqEnabled)
	setString(query, "tag_match", params.TagMatch)
	for _, id := range params.PositionIDs {
		query.Add("position_ids", id)
	}
	setBool(query, "active", params.Active)
	setBool(query, "closed", params.Closed)
	setBool(query, "archived", params.Archived)
	setBool(query, "resolved", params.Resolved)
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.Order != "" {
		query.Set("order", marketOrder(params.Order))
	}
	setBool(query, "ascending", params.Ascending)
	for _, id := range params.IDs {
		query.Add("id", id)
	}
	setString(query, "tag_id", params.TagID)
	setString(query, "event_id", params.EventID)
	addGammaFilterValues(query, "slug", params.Slug, params.Slugs)
	setBool(query, "negative_risk", params.NegativeRisk)
	setBool(query, "accepting_orders", params.AcceptingOrders)
	for _, id := range params.ClobTokenIDs {
		query.Add("clob_token_ids", id)
	}
	for _, id := range params.ConditionIDs {
		query.Add("condition_ids", id)
	}
	for _, addr := range params.MarketMakerAddress {
		query.Add("market_maker_address", addr)
	}
	setString(query, "liquidity_num_min", params.LiquidityNumMin)
	setString(query, "liquidity_num_max", params.LiquidityNumMax)
	setString(query, "volume_num_min", params.VolumeNumMin)
	setString(query, "volume_num_max", params.VolumeNumMax)
	setBool(query, "related_tags", params.RelatedTags)
	setBool(query, "cyom", params.CYOM)
	setString(query, "uma_resolution_status", params.UmaResolutionStatus)
	setString(query, "game_id", params.GameID)
	for _, marketType := range params.SportsMarketTypes {
		query.Add("sports_market_types", marketType)
	}
	setString(query, "rewards_min_size", params.RewardsMinSize)
	for _, id := range params.QuestionIDs {
		query.Add("question_ids", id)
	}
	setBool(query, "include_tag", params.IncludeTag)
	setString(query, "start_date_min", params.StartDateMin)
	setString(query, "start_date_max", params.StartDateMax)
	setString(query, "end_date_min", params.EndDateMin)
	setString(query, "end_date_max", params.EndDateMax)

	return query
}

// Only markets store their sortable volume and liquidity in numeric shadow columns.
func marketOrder(order string) string {
	tokens := strings.Split(order, ",")
	for i, token := range tokens {
		token = strings.TrimSpace(token)
		switch token {
		case "volume":
			token = "volumeNum"
		case "liquidity":
			token = "liquidityNum"
		}
		tokens[i] = token
	}
	return strings.Join(tokens, ",")
}

// IterMarkets returns an iterator for markets based on the provided filters.
func (c *Client) IterMarkets(ctx context.Context, p MarketFilterParams) iter.Seq2[Market, error] {
	return offsetItems(ctx, p.Limit, p.Offset, 100, -1, func(limit, offset int) ([]Market, error) {
		q := p
		q.Limit = limit
		q.Offset = offset
		return c.GetMarkets(ctx, q)
	}, nil, func(item Market) string { return string(item.ID) })
}

// GetMarketTags returns all tags associated with a market.
func (c *Client) GetMarketTags(ctx context.Context, marketID string) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(
		ctx,
		marketsEndpoint+"/"+url.PathEscape(marketID)+"/tags",
		nil,
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}
