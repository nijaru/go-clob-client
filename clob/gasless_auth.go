package clob

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

var ErrInvalidRelayerAuth = errors.New("invalid relayer authentication")

// RelayerAPIKey authenticates only the relayer, never CLOB or builder endpoints.
// Address identifies the API-key owner, which may differ from the wallet signer.
type RelayerAPIKey struct {
	Key     string
	Address common.Address
}

// RelayerAuthConfig selects exactly one authentication scheme. BuilderAuth may
// use a context-aware remote signer. It is distinct from CLOB L2 credentials.
type RelayerAuthConfig struct {
	APIKey      *RelayerAPIKey
	BuilderAuth BuilderAuth
}

type relayerAuthContextKey struct{}

// WithRelayerAuth configures relayer requests made with the returned context.
// An explicit selection replaces the client's BuilderAuth; headers from the two
// schemes are never combined. It does not change CLOB or RFQ authentication.
// Use the returned context for submission and subsequent handle waits.
func WithRelayerAuth(ctx context.Context, cfg RelayerAuthConfig) (context.Context, error) {
	if (cfg.APIKey == nil) == (cfg.BuilderAuth == nil) {
		return nil, fmt.Errorf("%w: select exactly one scheme", ErrInvalidRelayerAuth)
	}
	if cfg.APIKey != nil {
		key := *cfg.APIKey
		if strings.TrimSpace(key.Key) == "" || strings.ContainsAny(key.Key, "\r\n") ||
			key.Address == (common.Address{}) {
			return nil, fmt.Errorf("%w: API key or address", ErrInvalidRelayerAuth)
		}
		cfg.APIKey = &key // caller mutations cannot change request identity
	}
	return context.WithValue(ctx, relayerAuthContextKey{}, cfg), nil
}

func (c *AuthenticatedClient) relayerAuth(ctx context.Context) RelayerAuthConfig {
	if cfg, ok := ctx.Value(relayerAuthContextKey{}).(RelayerAuthConfig); ok {
		return cfg
	}
	return RelayerAuthConfig{BuilderAuth: c.getBuilderAuth()}
}
