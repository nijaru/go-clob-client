package gamma

import (
	"context"
	"iter"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// GetSports returns all sports metadata feeds.
func (c *Client) GetSports(ctx context.Context) ([]SportsMetadata, error) {
	var out []SportsMetadata
	err := c.http.GetJSON(ctx, sportsEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetTeams reads the first page of sports teams. Use IterTeams for a listing.
func (c *Client) GetTeams(ctx context.Context) ([]Team, error) {
	return c.GetTeamsPage(ctx, TeamFilterParams{})
}

// GetTeamsPage returns a single page of teams.
func (c *Client) GetTeamsPage(
	ctx context.Context,
	p TeamFilterParams,
) ([]Team, error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	query := gammaQuery(pageLimit(p.Limit, maxTeamsPageSize), p.Offset)
	addGammaFilterValues(query, "abbreviation", p.Abbreviation, p.Abbreviations)
	setBool(query, "ascending", p.Ascending)
	addGammaFilterValues(query, "league", p.League, p.Leagues)
	addGammaFilterValues(query, "name", p.Name, p.Names)
	setString(query, "order", p.Order)
	if len(p.ProviderIDs) > 0 {
		for _, id := range p.ProviderIDs {
			query.Add("provider_id", strconv.Itoa(id))
		}
	} else {
		setInt(query, "provider_id", p.ProviderID)
	}

	var out []Team
	err := c.http.GetJSON(ctx, teamsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// ListTeams returns all teams matching the provided filters.
func (c *Client) ListTeams(ctx context.Context, p TeamFilterParams) ([]Team, error) {
	return collect(c.IterTeams(ctx, p))
}

// IterTeams returns an iterator over teams.
func (c *Client) IterTeams(ctx context.Context, p TeamFilterParams) iter.Seq2[Team, error] {
	return offsetItems(
		ctx,
		p.Limit,
		p.Offset,
		maxTeamsPageSize,
		-1,
		func(limit, offset int) ([]Team, error) {
			q := p
			q.Limit = limit
			q.Offset = offset
			return c.GetTeamsPage(ctx, q)
		},
		nil,
		func(item Team) string { return strconv.Itoa(item.ID) },
	)
}

// GetMarketTypes returns the valid sports market types response.
func (c *Client) GetMarketTypes(ctx context.Context) (*SportsMarketTypesResponse, error) {
	var out SportsMarketTypesResponse
	err := c.http.GetJSON(ctx, sportsEndpoint+"/market-types", nil, polyhttp.AuthNone, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
