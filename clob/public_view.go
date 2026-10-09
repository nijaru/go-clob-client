package clob

import "context"

// AsPublic returns an independent unauthenticated view sharing transport,
// rate limiting and caches. It carries no signer, credentials or builder
// authentication. Existing views are unchanged; this is not API-key revocation.
func (c *Client) AsPublic() *Client {
	view := c.copyBase()
	view.http.Headers = nil
	view.geoblockHTTP.Headers = nil
	view.gatewayHTTP.Headers = nil
	return view
}

// Deauthenticate permanently joins this client's heartbeat lifecycle and
// returns a public view. It does not remotely revoke credentials, erase the
// signer's key or disable existing authenticated references. Go references
// are not consumed: discard authenticated views when their ownership ends.
func (c *AuthenticatedClient) Deauthenticate(ctx context.Context) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.Shutdown(ctx); err != nil {
		return nil, err
	}
	return c.Client.AsPublic(), nil
}
