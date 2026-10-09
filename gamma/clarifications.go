package gamma

import (
	"context"
	"iter"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func addGammaStateValues(query url.Values, p MarketClarificationsParams) {
	if len(p.States) > 0 {
		for _, state := range p.States {
			query.Add("state", string(state))
		}
		return
	}
	setString(query, "state", p.State)
}

// GetMarketClarifications returns one page of official market clarifications.
func (c *Client) GetMarketClarifications(
	ctx context.Context,
	p MarketClarificationsParams,
) ([]MarketClarification, error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	query := url.Values{}
	addGammaFilterValues(query, "market_id", p.MarketID, p.MarketIDs)
	addGammaFilterValues(query, "event_id", p.EventID, p.EventIDs)
	addGammaFilterValues(query, "question_id", p.QuestionID, p.QuestionIDs)
	addGammaStateValues(query, p)
	setBool(query, "show_in_frontend", p.ShowInFrontend)
	setString(query, "tx_hash", p.TxHash)
	setString(query, "order", p.Order)
	setBool(query, "ascending", p.Ascending)
	setInt(query, "limit", pageLimit(p.Limit, maxClarificationsPageSize))
	setInt(query, "offset", p.Offset)

	var out []MarketClarification
	err := c.http.GetJSON(ctx, marketClarificationsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// IterMarketClarifications walks all market clarifications using offset
// pagination, matching the official SDK's cursor abstraction.
func (c *Client) IterMarketClarifications(
	ctx context.Context,
	p MarketClarificationsParams,
) iter.Seq2[MarketClarification, error] {
	return offsetItems(
		ctx,
		p.Limit,
		p.Offset,
		maxClarificationsPageSize,
		-1,
		func(limit, offset int) ([]MarketClarification, error) {
			q := p
			q.Limit = limit
			q.Offset = offset
			return c.GetMarketClarifications(ctx, q)
		},
		nil,
		func(item MarketClarification) string { return strconv.Itoa(item.ID) },
	)
}
