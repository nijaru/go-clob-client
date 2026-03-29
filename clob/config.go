package clob

import (
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultHost is the production Polymarket CLOB base URL.
	DefaultHost = "https://clob.polymarket.com"
	// DefaultRTDSHost is the production Polymarket RTDS WebSocket URL.
	DefaultRTDSHost = "wss://rtds.polymarket.com"
	// DefaultGeoblockHost is the production Polymarket site host for geoblock checks.
	DefaultGeoblockHost = "https://polymarket.com"
	// PolygonChainID is the Polygon mainnet chain ID used by Polymarket.
	PolygonChainID = int64(137)
	defaultUA      = "go-clob-client/clob"
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
	// ChainID is the EVM chain ID. Defaults to PolygonChainID (137).
	ChainID int64
	// PrivateKey is the hex-encoded Ethereum private key used for signing.
	// Required for SignerClient and AuthenticatedClient.
	PrivateKey string
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

	// HeartbeatInterval is the duration between automatic heartbeats (2026 feature).
	// Defaults to 5 seconds.
	HeartbeatInterval time.Duration
	// DisableAutoHeartbeat prevents the client from starting the background heartbeat loop.
	DisableAutoHeartbeat bool

	// TickSizeCacheTTL is the duration for which tick sizes are cached.
	// Defaults to 0 (no expiration).
	TickSizeCacheTTL time.Duration

	// RPCURL is the Ethereum JSON-RPC endpoint used for on-chain CTF operations
	// (split, merge, redeem). Defaults to "https://polygon-rpc.com".
	RPCURL string
	// RetryMax is the maximum number of times to retry a failed request.
	// Defaults to 0 (no retries).
	RetryMax int
	// RetryBackoff is the base duration for exponential backoff between retries.
	// Defaults to 1 second.
	RetryBackoff time.Duration
	// RateLimit is the maximum number of requests per second.
	// Defaults to 5 req/s. Set to 0 to disable rate limiting.
	RateLimit float64
	// RateBurst is the maximum burst size for the rate limiter.
	// Defaults to 10.
	RateBurst int
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
