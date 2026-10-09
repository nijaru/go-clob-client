package clob

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/nijaru/go-clob-client/clob/ws/rtds"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Client is the base Polymarket CLOB client containing public, unauthenticated methods.
type Client struct {
	host                 string
	rtdsHost             string
	relayerHost          string
	collateralReturnHost string
	builderGatewayHost   string
	chainID              int64
	useServerTime        bool
	http                 *polyhttp.Client
	geoblockHTTP         *polyhttp.Client
	gatewayHTTP          *polyhttp.Client
	rpcURL               string

	*clientCacheState

	cacheTTL     time.Duration
	retryMax     int
	retryBackoff time.Duration
	rateLimiter  *rate.Limiter
}

// SignerClient extends the base client with methods requiring an Ethereum signer (L1).
type SignerClient struct {
	*Client
	signer        *polyauth.Signer
	signatureType SignatureType
	funderAddress string
	saltGenerator func() (uint64, error)
}

// AuthenticatedClient extends the base client with methods requiring API credentials (L2).
type AuthenticatedClient struct {
	*SignerClient
	authMu            sync.RWMutex
	creds             *Credentials
	decodedSecret     []byte
	builderAuth       BuilderAuth
	heartbeatID       string
	heartbeatInterval time.Duration
	heartbeatCancel   context.CancelFunc
	heartbeatDone     chan struct{}
	heartbeatMu       sync.Mutex
	heartbeatClosed   bool
}

// NewClient creates a read-only CLOB client. No private key or credentials are required.
func NewClient(config Config) (*Client, error) {
	config = config.normalized()
	if err := config.validate(); err != nil {
		return nil, err
	}
	return newBase(config), nil
}

// NewSignerClient creates a signing CLOB client with L1 Ethereum auth. PrivateKey is required.
func NewSignerClient(config Config) (*SignerClient, error) {
	if config.PrivateKey == "" {
		return nil, fmt.Errorf("PrivateKey is required")
	}
	config = config.normalized()
	if err := config.validate(); err != nil {
		return nil, err
	}
	return newSignerFrom(newBase(config), config)
}

// NewAuthenticatedClient creates a fully authenticated CLOB client with L2 API key auth.
// Both PrivateKey and Credentials are required. Construction performs no network
// requests and starts no goroutines; call StartHeartbeats explicitly if needed.
func NewAuthenticatedClient(config Config) (*AuthenticatedClient, error) {
	if config.PrivateKey == "" {
		return nil, fmt.Errorf("PrivateKey is required")
	}
	if config.Credentials == nil {
		return nil, fmt.Errorf("Credentials are required")
	}
	config = config.normalized()
	if err := config.validate(); err != nil {
		return nil, err
	}
	base := newBase(config)
	sc, err := newSignerFrom(base, config)
	if err != nil {
		return nil, err
	}
	decodedSecret, err := decodeCredentials(*config.Credentials)
	if err != nil {
		return nil, fmt.Errorf("invalid API secret: %w", err)
	}
	authClient := &AuthenticatedClient{
		SignerClient:      sc,
		creds:             new(*config.Credentials),
		decodedSecret:     decodedSecret,
		builderAuth:       config.BuilderAuth,
		heartbeatInterval: config.HeartbeatInterval,
	}
	base.http.Headers = authClient.addAuthHeaders
	base.gatewayHTTP.Headers = authClient.addAuthHeaders
	return authClient, nil
}

func newBase(config Config) *Client {
	base := &Client{
		host:                 config.Host,
		rtdsHost:             config.RTDSHost,
		relayerHost:          config.RelayerHost,
		collateralReturnHost: config.CollateralReturnHost,
		builderGatewayHost:   config.BuilderGatewayHost,
		chainID:              config.ChainID,
		useServerTime:        config.UseServerTime,
		rpcURL:               config.RPCURL,
		clientCacheState:     newClientCacheState(),

		cacheTTL:     config.TickSizeCacheTTL,
		retryMax:     config.RetryMax,
		retryBackoff: config.RetryBackoff,
		rateLimiter:  newLimiter(config.RateLimit, config.RateBurst),
	}
	base.http = &polyhttp.Client{
		BaseURL:           config.Host,
		HTTPClient:        config.HTTPClient,
		UserAgent:         config.UserAgent,
		OnRateLimitUpdate: config.OnRateLimitUpdate,
	}
	base.geoblockHTTP = &polyhttp.Client{
		BaseURL:           config.GeoblockHost,
		HTTPClient:        config.HTTPClient,
		UserAgent:         config.UserAgent,
		OnRateLimitUpdate: config.OnRateLimitUpdate,
	}
	base.gatewayHTTP = &polyhttp.Client{
		BaseURL:           config.BuilderGatewayHost,
		HTTPClient:        config.HTTPClient,
		UserAgent:         config.UserAgent,
		OnRateLimitUpdate: config.OnRateLimitUpdate,
	}
	return base
}

func newSignerFrom(base *Client, config Config) (*SignerClient, error) {
	if _, err := getContractConfig(config.ChainID); err != nil {
		return nil, err
	}
	signer, err := polyauth.ParsePrivateKey(config.PrivateKey)
	if err != nil {
		return nil, err
	}
	funderAddress, err := normalizeFunderAddress(
		config.ChainID,
		signer.Address().Hex(),
		config.SignatureType,
		config.FunderAddress,
	)
	if err != nil {
		return nil, err
	}
	sc := &SignerClient{
		Client:        base,
		signer:        signer,
		signatureType: config.SignatureType,
		funderAddress: funderAddress,
		saltGenerator: generateSalt,
	}
	base.http.Headers = sc.addAuthHeaders
	base.gatewayHTTP.Headers = sc.addAuthHeaders
	return sc, nil
}

func (c *Client) copyBase() *Client {
	// Share cache state, but give each auth view an independent header resolver.
	// Preserve the underlying HTTP client and response callbacks.
	copy := *c
	copy.http = cloneTransport(c.http)
	copy.geoblockHTTP = cloneTransport(c.geoblockHTTP)
	copy.gatewayHTTP = cloneTransport(c.gatewayHTTP)
	return &copy
}

// NewRTDSClient creates a new RTDS (Real-Time Data Stream) WebSocket client.
func (c *Client) NewRTDSClient() *rtds.Client {
	return rtds.NewClient(c.rtdsHost, nil)
}

// AsSigner upgrades a base client to a SignerClient.
func (c *Client) AsSigner(
	privateKey string,
	sigType SignatureType,
	funder string,
) (*SignerClient, error) {
	signer, err := polyauth.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	funderAddress, err := normalizeFunderAddress(c.chainID, signer.Address().Hex(), sigType, funder)
	if err != nil {
		return nil, err
	}

	sc := &SignerClient{
		Client:        c.copyBase(),
		signer:        signer,
		signatureType: sigType,
		funderAddress: funderAddress,
		saltGenerator: generateSalt,
	}
	sc.http.Headers = sc.addAuthHeaders
	sc.gatewayHTTP.Headers = sc.addAuthHeaders
	return sc, nil
}

// AsAuthenticated upgrades a SignerClient to an AuthenticatedClient.
func (c *SignerClient) AsAuthenticated(
	creds Credentials,
	builder BuilderAuth,
) (*AuthenticatedClient, error) {
	return c.AsAuthenticatedWithInterval(creds, builder, 5*time.Second)
}

// AsAuthenticatedWithInterval upgrades a SignerClient to an AuthenticatedClient
// with a configurable heartbeat interval. Use this when upgrading from an existing
// SignerClient where the original Config is no longer available.
func (c *SignerClient) AsAuthenticatedWithInterval(
	creds Credentials,
	builder BuilderAuth,
	heartbeatInterval time.Duration,
) (*AuthenticatedClient, error) {
	decodedSecret, err := decodeCredentials(creds)
	if err != nil {
		return nil, fmt.Errorf("invalid API secret: %w", err)
	}

	if heartbeatInterval < 0 {
		return nil, fmt.Errorf("heartbeat interval must not be negative")
	}
	if heartbeatInterval == 0 {
		heartbeatInterval = 5 * time.Second
	}

	ac := &AuthenticatedClient{
		SignerClient: &SignerClient{
			Client:        c.Client.copyBase(),
			signer:        c.signer,
			signatureType: c.signatureType,
			funderAddress: c.funderAddress,
			saltGenerator: c.saltGenerator,
		},
		creds:             &creds,
		decodedSecret:     decodedSecret,
		builderAuth:       builder,
		heartbeatInterval: heartbeatInterval,
	}
	ac.http.Headers = ac.addAuthHeaders
	ac.gatewayHTTP.Headers = ac.addAuthHeaders
	return ac, nil
}

// NewAuthenticatedRTDSClient creates a new RTDS client that can also subscribe to authenticated topics.
func (c *AuthenticatedClient) NewAuthenticatedRTDSClient() *rtds.Client {
	creds := c.credentials()
	rtdsCreds := &rtds.Credentials{
		Key:        creds.Key,
		Secret:     creds.Secret,
		Passphrase: creds.Passphrase,
	}
	return rtds.NewClient(c.rtdsHost, nil).WithCredentials(rtdsCreds)
}

// ClearTickSizeCache removes the cached tick size, negative risk flag, and
// order metadata for a specific token.
func (c *Client) ClearTickSizeCache(tokenID string) {
	c.tickSizeMu.Lock()
	c.tickSizeGeneration++
	delete(c.tickSizeCache, tokenID)
	delete(c.tickSizeTimestamps, tokenID)
	c.tickSizeMu.Unlock()

	c.negRiskMu.Lock()
	c.negRiskGeneration++
	delete(c.negRiskCache, tokenID)
	delete(c.negRiskTimestamps, tokenID)
	c.negRiskMu.Unlock()
	c.clearOrderMetadata(tokenID)
}

// ClearNegRiskCache removes the cached negative risk flag for a specific token.
func (c *Client) ClearNegRiskCache(tokenID string) {
	c.negRiskMu.Lock()
	c.negRiskGeneration++
	delete(c.negRiskCache, tokenID)
	delete(c.negRiskTimestamps, tokenID)
	c.negRiskMu.Unlock()
	c.clearOrderMetadata(tokenID)
}

// SetTickSize pre-populates the tick size cache for tokenID, bypassing the HTTP fetch.
func (c *Client) SetTickSize(tokenID string, size TickSize) {
	now := time.Now()
	c.tickSizeMu.Lock()
	c.tickSizeGeneration++
	c.tickSizeCache[tokenID] = size
	c.tickSizeTimestamps[tokenID] = now
	c.tickSizeMu.Unlock()
}

// SetNegRisk pre-populates the neg risk cache for tokenID.
func (c *Client) SetNegRisk(tokenID string, negRisk bool) {
	now := time.Now()
	c.negRiskMu.Lock()
	c.negRiskGeneration++
	c.negRiskCache[tokenID] = negRisk
	c.negRiskTimestamps[tokenID] = now
	c.negRiskMu.Unlock()
}

// InvalidateCaches clears all internal caches (tick size, neg risk, order
// market metadata, and builder fee rates).
func (c *Client) InvalidateCaches() {
	c.tickSizeMu.Lock()
	c.tickSizeGeneration++
	clear(c.tickSizeCache)
	clear(c.tickSizeTimestamps)
	c.tickSizeMu.Unlock()

	c.negRiskMu.Lock()
	c.negRiskGeneration++
	clear(c.negRiskCache)
	clear(c.negRiskTimestamps)
	c.negRiskMu.Unlock()
	c.clearAllOrderMetadata()
	c.invalidateServerVersion()
}

// Host returns the base CLOB API host for the client.
func (c *Client) Host() string {
	return c.host
}

// resolveContractConfig returns the contract config for the client's chain.
func (c *Client) resolveContractConfig(negRisk bool) (contractConfig, error) {
	return getContractConfig(c.chainID)
}

func newLimiter(r float64, b int) *rate.Limiter {
	if r <= 0 {
		return rate.NewLimiter(rate.Inf, 0)
	}
	if b <= 0 {
		b = 1
	}
	return rate.NewLimiter(rate.Limit(r), b)
}
