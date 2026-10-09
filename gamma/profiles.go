package gamma

import (
	"context"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp" // GetPublicProfile returns the public profile for a wallet address.
)

func (c *Client) GetPublicProfile(ctx context.Context, address string) (*PublicProfile, error) {
	query := url.Values{}
	query.Set("address", address)

	var out *PublicProfile
	err := c.http.GetJSON(ctx, profileEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}
