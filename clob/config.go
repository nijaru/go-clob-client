package clob

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/signing"
)

const (
	// DefaultHost is the production Polymarket CLOB base URL.
	DefaultHost = "https://clob-v2.polymarket.com"
	// DefaultRTDSHost is the production Polymarket RTDS WebSocket URL.
	DefaultRTDSHost = "wss://ws-live-data.polymarket.com"
	// DefaultGeoblockHost is the production Polymarket site host for geoblock checks.
	DefaultGeoblockHost = "https://polymarket.com"
	// DefaultRelayerHost is the production Polymarket gasless relayer URL.
	DefaultRelayerHost = "https://relayer-v2.polymarket.com"
	// DefaultBuilderGatewayHost is the production Polymarket builder gateway
	// URL used for requester-side combo RFQ requests and accepts.
	DefaultBuilderGatewayHost = "https://combos-rfq-gateway-builder.polymarket.com"
	// DefaultCollateralReturnHost is the production collateral-return service URL.
	DefaultCollateralReturnHost = "https://combos-rfq-collateral-return.polymarket.com"
	// PolygonChainID is the Polygon mainnet chain ID used by Polymarket.
	PolygonChainID = int64(137)
	// AmoyChainID is the Polygon Amoy testnet chain ID.
	AmoyChainID = int64(80002)
	defaultUA   = "go-clob-client/clob"
)

// SignatureType controls which signer/funder model Polymarket should expect for the account.
type SignatureType int

const (
	// SignatureTypeEOA signs orders directly from an externally owned account.
	SignatureTypeEOA SignatureType = iota
	// SignatureTypePolyProxy uses the Polymarket proxy-wallet signer model.
	SignatureTypePolyProxy
	// SignatureTypePolyGnosisSafe uses the Polymarket safe-based signer model.
	SignatureTypePolyGnosisSafe
	// SignatureTypePoly1271 uses EIP-1271 smart contract wallet signatures (V2 orders only).
	SignatureTypePoly1271

	// SignatureTypeMagic is the legacy name for SignatureTypePolyProxy.
	SignatureTypeMagic = SignatureTypePolyProxy
	// SignatureTypeBrowserProxy is the legacy name for SignatureTypePolyGnosisSafe.
	SignatureTypeBrowserProxy = SignatureTypePolyGnosisSafe
)

// Config configures a Polymarket CLOB client.
type Config struct {
	// Host is the CLOB API base URL. Defaults to DefaultHost.
	Host string
	// RTDSHost overrides the host used for RTDS WebSocket connections.
	RTDSHost string
	// GeoblockHost overrides the host used for geoblock checks.
	GeoblockHost string
	// RelayerHost overrides the gasless relayer URL. Defaults to DefaultRelayerHost.
	RelayerHost string
	// CollateralReturnHost overrides the collateral-return service URL.
	// Defaults to DefaultCollateralReturnHost.
	CollateralReturnHost string
	// BuilderGatewayHost overrides the builder gateway URL used for
	// requester-side combo RFQ requests and accepts. Defaults to
	// DefaultBuilderGatewayHost.
	BuilderGatewayHost string
	// ChainID is the EVM chain ID. Defaults to PolygonChainID (137).
	ChainID int64
	// PrivateKey is a local signing convenience. Set exactly one of PrivateKey
	// and Signer for SignerClient or AuthenticatedClient.
	PrivateKey string
	// Signer supports external or hardware EOA signing without exporting a key.
	Signer signing.Signer
	// Credentials are the Polymarket API credentials for L2 authenticated requests.
	// Required for AuthenticatedClient.
	Credentials *Credentials
	// BuilderAuth enables builder-authenticated endpoints. Optional.
	BuilderAuth BuilderAuth
	// SignatureType selects the wallet model Polymarket uses to verify signatures.
	// Defaults to SignatureTypeEOA.
	SignatureType SignatureType
	// FunderAddress overrides the address that holds funds on Polymarket.
	// Required for proxy/Magic wallet users; derived automatically for EOA wallets.
	FunderAddress string
	// HTTPClient overrides the default HTTP client. Defaults to a 15-second timeout client.
	HTTPClient *http.Client
	// UserAgent sets the User-Agent header on all requests.
	UserAgent string
	// UseServerTime fetches the server timestamp for each authenticated request
	// instead of using local time. Useful when local clock skew causes auth failures.
	UseServerTime bool

	// HeartbeatInterval is the posting interval used by StartHeartbeats.
	// Defaults to 5 seconds. Constructors never start heartbeats implicitly.
	HeartbeatInterval time.Duration

	// TickSizeCacheTTL is the duration for which tick sizes are cached.
	// Defaults to 0 (no expiration).
	TickSizeCacheTTL time.Duration

	// RPCURL is the Ethereum JSON-RPC endpoint used for on-chain CTF operations
	// (split, merge, redeem). Defaults to "https://polygon-rpc.com".
	RPCURL string
	// RetryMax is the maximum number of retries for retry-eligible requests.
	// Today that means plain GET requests issued through the shared HTTP helpers.
	// Defaults to 0 (no retries).
	RetryMax int
	// RetryBackoff is the base duration for exponential backoff between retries
	// for retry-eligible requests.
	// Defaults to 1 second.
	RetryBackoff time.Duration
	// RateLimit is the maximum number of requests per second.
	// Defaults to 5 req/s. Set to a negative value to disable rate limiting.
	RateLimit float64
	// RateBurst is the maximum burst size for the rate limiter.
	// Defaults to 10.
	RateBurst int
	// OnRateLimitUpdate is invoked with the parsed Poly-RateLimit-* state
	// whenever a response reports it, both successful and failed. Errors
	// raised by the listener are ignored and must not affect request
	// handling.
	OnRateLimitUpdate func(*RateLimitUpdate)
}

func (c Config) validate() error {
	if c.PrivateKey != "" && c.Signer != nil {
		return fmt.Errorf("set exactly one of PrivateKey and Signer")
	}
	if c.RetryMax < 0 || c.RetryBackoff < 0 || c.TickSizeCacheTTL < 0 || c.HeartbeatInterval < 0 ||
		c.RateBurst < 0 {
		return fmt.Errorf("retry, cache, heartbeat, and burst settings must not be negative")
	}
	if math.IsNaN(c.RateLimit) || math.IsInf(c.RateLimit, 0) {
		return fmt.Errorf("RateLimit must be finite")
	}
	for _, host := range []string{c.Host, c.GeoblockHost, c.RelayerHost, c.BuilderGatewayHost, c.CollateralReturnHost} {
		u, err := url.Parse(host)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("invalid HTTP host %q", host)
		}
	}
	u, err := url.Parse(c.RTDSHost)
	if err != nil || (u.Scheme != "wss" && u.Scheme != "ws") || u.Host == "" {
		return fmt.Errorf("invalid RTDS host %q", c.RTDSHost)
	}
	return nil
}

func (c Config) normalized() Config {
	if c.Host == "" {
		c.Host = DefaultHost
	}
	c.Host = strings.TrimRight(c.Host, "/")

	if c.RTDSHost == "" {
		c.RTDSHost = DefaultRTDSHost
	}
	c.RTDSHost = strings.TrimRight(c.RTDSHost, "/")

	if c.GeoblockHost == "" {
		c.GeoblockHost = DefaultGeoblockHost
	}
	c.GeoblockHost = strings.TrimRight(c.GeoblockHost, "/")

	if c.RelayerHost == "" {
		c.RelayerHost = DefaultRelayerHost
	}
	c.RelayerHost = strings.TrimRight(c.RelayerHost, "/")

	if c.CollateralReturnHost == "" {
		c.CollateralReturnHost = DefaultCollateralReturnHost
	}
	c.CollateralReturnHost = strings.TrimRight(c.CollateralReturnHost, "/")

	if c.BuilderGatewayHost == "" {
		c.BuilderGatewayHost = DefaultBuilderGatewayHost
	}
	c.BuilderGatewayHost = strings.TrimRight(c.BuilderGatewayHost, "/")

	if c.ChainID == 0 {
		c.ChainID = PolygonChainID
	}

	if c.RPCURL == "" {
		c.RPCURL = "https://polygon-rpc.com"
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}

	if c.UserAgent == "" {
		c.UserAgent = defaultUA
	}

	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 5 * time.Second
	}

	if c.RetryBackoff == 0 {
		c.RetryBackoff = 1 * time.Second
	}

	if c.RateLimit == 0 {
		c.RateLimit = 5
	}
	if c.RateBurst == 0 {
		c.RateBurst = 10
	}

	return c
}
