package gamma

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func polymarketSlug(raw, resource string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" ||
		(parsed.Hostname() != "polymarket.com" && parsed.Hostname() != "www.polymarket.com") {
		return "", fmt.Errorf("gamma: expected a valid Polymarket HTTPS URL")
	}
	segments := strings.FieldsFunc(parsed.Path, func(r rune) bool { return r == '/' })
	if resource == "market" && len(segments) >= 2 && len(segments) <= 3 && segments[0] == "event" {
		return segments[len(segments)-1], nil
	}
	if len(segments) != 2 || segments[0] != resource {
		return "", fmt.Errorf("gamma: expected a Polymarket %s URL", resource)
	}
	return segments[1], nil
}

// GetMarketByURL resolves /market/slug, /event/slug and /event/slug/market-slug
// URLs to a Gamma market lookup; the supplied URL is never fetched directly.
func (c *Client) GetMarketByURL(
	ctx context.Context,
	rawURL string,
	options ...MarketOptions,
) (*Market, error) {
	slug, err := polymarketSlug(rawURL, "market")
	if err != nil {
		return nil, err
	}
	return c.GetMarketBySlug(ctx, slug, options...)
}

// GetEventByURL resolves an official Polymarket event URL to its Gamma slug.
func (c *Client) GetEventByURL(
	ctx context.Context,
	rawURL string,
	options ...EventOptions,
) (*Event, error) {
	slug, err := polymarketSlug(rawURL, "event")
	if err != nil {
		return nil, err
	}
	return c.GetEventBySlug(ctx, slug, options...)
}
