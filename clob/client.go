package clob

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/nijaru/go-clob-client/clob/ws/rtds"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Client is the base Polymarket CLOB client containing public, unauthenticated methods.
type Client struct {
	host          string
	rtdsHost      string
	chainID       int64
	useServerTime bool
	http          *polyhttp.Client
	geoblockHTTP  *polyhttp.Client

	tickSizeMu         *sync.RWMutex
	tickSizeCache      map[string]TickSize
	tickSizeTimestamps map[string]time.Time
	negRiskMu          *sync.RWMutex
	negRiskCache       map[string]bool
	negRiskTimestamps  map[string]time.Time
	feeRateMu          *sync.RWMutex
	feeRateCache       map[string]int64
	feeRateTimestamps  map[string]time.Time

	tickSizeTTL  time.Duration
	retryMax     int
	retryBackoff time.Duration
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
	authMu           sync.RWMutex
	creds             *Credentials
	builderAuth       BuilderAuth
	heartbeatID       string
	heartbeatInterval time.Duration
	heartbeatCancel   context.CancelFunc
	heartbeatDone     chan struct{}
}

// NewClient creates a read-only CLOB client. No private key or credentials are required.
func NewClient(config Config) (*Client, error) {
	config = config.normalized()
	return newBase(config), nil
}

// NewSignerClient creates a signing CLOB client with L1 Ethereum auth. PrivateKey is required.
func NewSignerClient(config Config) (*SignerClient, error) {
	if config.PrivateKey == "" {
		return nil, fmt.Errorf("PrivateKey is required")
	}
	config = config.normalized()
	return newSignerFrom(newBase(config), config)
}

// NewAuthenticatedClient creates a fully authenticated CLOB client with L2 API key auth.
// Both PrivateKey and Credentials are required.
func NewAuthenticatedClient(config Config) (*AuthenticatedClient, error) {
	if config.PrivateKey == "" {
		return nil, fmt.Errorf("PrivateKey is required")
	}
	if config.Credentials == nil {
		return nil, fmt.Errorf("Credentials are required")
	}
	config = config.normalized()
	base := newBase(config)
	sc, err := newSignerFrom(base, config)
	if err != nil {
		return nil, err
	}
	authClient := &AuthenticatedClient{
		SignerClient:      sc,
		creds:             config.Credentials,
		builderAuth:       config.BuilderAuth,
		heartbeatInterval: config.HeartbeatInterval,
	}
	base.http.Headers = authClient.addAuthHeaders
	if !config.DisableAutoHeartbeat {
		authClient.startHeartbeatLoop()
	}
	return authClient, nil
}

// Credentials returns the current API credentials.
func (c *AuthenticatedClient) Credentials() *Credentials {
	return c.credentials()
}

// PromoteToBuilder upgrades the client with builder credentials, enabling builder-authenticated requests.
func (c *AuthenticatedClient) PromoteToBuilder(auth BuilderAuth) {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	c.builderAuth = auth
}

func newBase(config Config) *Client {
	base := &Client{
		host:          config.Host,
		rtdsHost:      config.RTDSHost,
		chainID:       config.ChainID,
		useServerTime: config.UseServerTime,
		tickSizeMu:         &sync.RWMutex{},
		tickSizeCache:      make(map[string]TickSize),
		tickSizeTimestamps: make(map[string]time.Time),
		negRiskMu:          &sync.RWMutex{},
		negRiskCache:       make(map[string]bool),
		negRiskTimestamps:  make(map[string]time.Time),
		feeRateMu:          &sync.RWMutex{},
		feeRateCache:       make(map[string]int64),
		feeRateTimestamps:  make(map[string]time.Time),

		tickSizeTTL:  config.TickSizeCacheTTL,
		retryMax:     config.RetryMax,
		retryBackoff: config.RetryBackoff,
	}
	base.http = &polyhttp.Client{
		BaseURL:    config.Host,
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
	}
	base.geoblockHTTP = &polyhttp.Client{
		BaseURL:    config.GeoblockHost,
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
	}
	return base
}

func newSignerFrom(base *Client, config Config) (*SignerClient, error) {
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
	return sc, nil
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

	// Deep copy the base client to prevent mutating the shared http transport
	baseCopy := *c
	baseCopy.http = &polyhttp.Client{
		BaseURL:    c.http.BaseURL,
		HTTPClient: c.http.HTTPClient,
		UserAgent:  c.http.UserAgent,
		Headers:    nil, // Will be set below
	}
	baseCopy.geoblockHTTP = &polyhttp.Client{
		BaseURL:    c.geoblockHTTP.BaseURL,
		HTTPClient: c.geoblockHTTP.HTTPClient,
		UserAgent:  c.geoblockHTTP.UserAgent,
	}

	sc := &SignerClient{
		Client:        &baseCopy,
		signer:        signer,
		signatureType: sigType,
		funderAddress: funderAddress,
		saltGenerator: generateSalt,
	}
	sc.http.Headers = sc.addAuthHeaders
	return sc, nil
}

// AsAuthenticated upgrades a SignerClient to an AuthenticatedClient.
func (c *SignerClient) AsAuthenticated(
	creds Credentials,
	builder BuilderAuth,
) *AuthenticatedClient {
	// Deep copy the base client to prevent mutating the shared http transport
	baseCopy := *(c.Client)
	baseCopy.http = &polyhttp.Client{
		BaseURL:    c.http.BaseURL,
		HTTPClient: c.http.HTTPClient,
		UserAgent:  c.http.UserAgent,
		Headers:    nil, // Will be set below
	}
	baseCopy.geoblockHTTP = &polyhttp.Client{
		BaseURL:    c.geoblockHTTP.BaseURL,
		HTTPClient: c.geoblockHTTP.HTTPClient,
		UserAgent:  c.geoblockHTTP.UserAgent,
	}

	signerCopy := *c
	signerCopy.Client = &baseCopy

	ac := &AuthenticatedClient{
		SignerClient: &signerCopy,
		creds:        &creds,
		builderAuth:  builder,
	}
	ac.http.Headers = ac.addAuthHeaders
	return ac
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

// Close stops any background tasks (like heartbeats) and cleans up resources.
func (c *AuthenticatedClient) Close() error {
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
		<-c.heartbeatDone
	}
	return nil
}

func (c *AuthenticatedClient) startHeartbeatLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	c.heartbeatCancel = cancel
	c.heartbeatDone = make(chan struct{})

	go func() {
		defer close(c.heartbeatDone)
		ticker := time.NewTicker(c.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := c.PostHeartbeat(ctx, c.heartbeatID)
				if err == nil {
					c.heartbeatID = resp.HeartbeatID
				}
			}
		}
	}()
}

// ClearTickSizeCache removes the cached tick size, negative risk flag, and fee rate for a specific token.
func (c *Client) ClearTickSizeCache(tokenID string) {
	c.tickSizeMu.Lock()
	delete(c.tickSizeCache, tokenID)
	c.tickSizeMu.Unlock()

	c.negRiskMu.Lock()
	delete(c.negRiskCache, tokenID)
	c.negRiskMu.Unlock()

	c.feeRateMu.Lock()
	delete(c.feeRateCache, tokenID)
	c.feeRateMu.Unlock()
}

// ClearTickSizeCaches clears all cached tick sizes, negative risk flags, and fee rates.
func (c *Client) ClearTickSizeCaches() {
	c.tickSizeMu.Lock()
	c.tickSizeCache = make(map[string]TickSize)
	c.tickSizeMu.Unlock()

	c.negRiskMu.Lock()
	c.negRiskCache = make(map[string]bool)
	c.negRiskMu.Unlock()

	c.feeRateMu.Lock()
	c.feeRateCache = make(map[string]int64)
	c.feeRateMu.Unlock()
}

// ClearFeeRateCache removes the cached fee rate for a specific token.
func (c *Client) ClearFeeRateCache(tokenID string) {
	c.feeRateMu.Lock()
	delete(c.feeRateCache, tokenID)
	c.feeRateMu.Unlock()
}

// ClearNegRiskCache removes the cached negative risk flag for a specific token.
func (c *Client) ClearNegRiskCache(tokenID string) {
	c.negRiskMu.Lock()
	delete(c.negRiskCache, tokenID)
	c.negRiskMu.Unlock()
}

// Host returns the base CLOB API host for the client.
func (c *Client) Host() string {
	return c.host
}

// SetCredentials updates the API credentials used for authenticated requests.
// Safe to call concurrently with in-flight requests.
func (c *AuthenticatedClient) SetCredentials(creds Credentials) {
	c.authMu.Lock()
	c.creds = &creds
	c.authMu.Unlock()
}

// Address returns the signer address backing the client.
func (c *SignerClient) Address() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

// credentials returns the current credentials under a read lock.
func (c *AuthenticatedClient) credentials() *Credentials {
	c.authMu.RLock()
	creds := c.creds
	c.authMu.RUnlock()
	return creds
}

func (c *AuthenticatedClient) getBuilderAuth() BuilderAuth {
	c.authMu.RLock()
	defer c.authMu.RUnlock()
	return c.builderAuth
}

func (c *SignerClient) addAuthHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	level polyhttp.AuthLevel,
	nonce *int64,
) (map[string]string, error) {
	switch level {
	case polyhttp.AuthNone:
		return nil, nil
	case polyhttp.AuthL1:
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		value := int64(0)
		if nonce != nil {
			value = *nonce
		}
		return polyauth.L1Headers(c.signer, c.chainID, timestamp, value)
	default:
		return nil, fmt.Errorf(
			"this client only supports L1 auth, please upgrade to an AuthenticatedClient",
		)
	}
}

func (c *AuthenticatedClient) addAuthHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	level polyhttp.AuthLevel,
	nonce *int64,
) (map[string]string, error) {
	switch level {
	case polyhttp.AuthNone, polyhttp.AuthL1:
		return c.SignerClient.addAuthHeaders(ctx, method, path, body, level, nonce)
	case polyhttp.AuthL2:
		creds := c.credentials()
		if creds == nil {
			return nil, fmt.Errorf("level 2 auth requires API credentials")
		}
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		return polyauth.L2Headers(
			c.signer,
			creds.Key,
			creds.Secret,
			creds.Passphrase,
			timestamp,
			method,
			path,
			body,
		)
	case polyhttp.AuthL2Builder:
		creds := c.credentials()
		if creds == nil {
			return nil, fmt.Errorf("level 2 auth requires API credentials")
		}
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		headers, err := polyauth.L2Headers(
			c.signer,
			creds.Key,
			creds.Secret,
			creds.Passphrase,
			timestamp,
			method,
			path,
			body,
		)
		if err != nil {
			return nil, err
		}
		if c.getBuilderAuth() == nil {
			return headers, nil
		}
		builderHeaders, err := c.builderHeaders(ctx, method, path, body, timestamp)
		if err != nil {
			return nil, err
		}
		maps.Copy(headers, builderHeaders)
		return headers, nil
	default:
		return nil, fmt.Errorf("unknown auth level %d", level)
	}
}

func (c *Client) timestamp(ctx context.Context) (int64, error) {
	if !c.useServerTime {
		return time.Now().Unix(), nil
	}

	var serverTime int64
	if err := c.http.GetJSON(ctx, timeEndpoint, nil, polyhttp.AuthNone, &serverTime); err != nil {
		return 0, err
	}
	return serverTime, nil
}

func (c *Client) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.GetJSON(ctx, path, query, auth, out)
	})
}

func (c *Client) getGeoblockJSON(
	ctx context.Context,
	path string,
	query url.Values,
	out any,
) error {
	return c.geoblockHTTP.GetJSON(ctx, path, query, polyhttp.AuthNone, out)
}

func (c *Client) postJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.PostJSON(ctx, path, body, auth, out)
	})
}

func (c *Client) deleteJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.DeleteJSON(ctx, path, body, auth, out)
	})
}

func (c *Client) deleteJSONQuery(
	ctx context.Context,
	path string,
	query url.Values,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.DeleteJSONQuery(ctx, path, query, body, auth, out)
	})
}

func (c *Client) getJSONWithNonce(
	ctx context.Context,
	path string,
	query url.Values,
	auth polyhttp.AuthLevel,
	nonce int64,
	out any,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.GetJSONWithNonce(ctx, path, query, auth, nonce, out)
	})
}

func (c *Client) postJSONWithNonce(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	nonce int64,
	out any,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.PostJSONWithNonce(ctx, path, body, auth, nonce, out)
	})
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	auth polyhttp.AuthLevel,
	out any,
	extraHeaders map[string]string,
) error {
	return c.withRetry(ctx, func() error {
		return c.http.DoJSON(ctx, method, path, query, body, auth, nil, extraHeaders, out)
	})
}

func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for i := 0; i <= c.retryMax; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		// Only retry on:
		// 1. Connection/context errors (non-API errors)
		// 2. HTTP 429 (Rate Limit)
		// 3. HTTP 5xx (Server Error)
		var apiErr *polyhttp.APIError
		shouldRetry := false
		if errors.As(err, &apiErr) {
			if apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500 {
				shouldRetry = true
			}
		} else {
			// Not an API error (e.g., timeout, connection refused) - safe to retry
			shouldRetry = true
		}

		if !shouldRetry {
			return err
		}

		if i < c.retryMax {
			backoff := c.retryBackoff * time.Duration(1<<i)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
	}
	return lastErr
}

func (c *AuthenticatedClient) builderHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	timestamp int64,
) (map[string]string, error) {
	if c.getBuilderAuth() == nil {
		return nil, fmt.Errorf("builder auth requires Config.BuilderAuth")
	}

	return c.getBuilderAuth().Headers(ctx, BuilderHeaderRequest{
		Method:    method,
		Path:      path,
		Body:      body,
		Timestamp: timestamp,
	})
}

func (c *AuthenticatedClient) builderOnlyHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
) (map[string]string, error) {
	timestamp, err := c.timestamp(ctx)
	if err != nil {
		return nil, err
	}
	return c.builderHeaders(ctx, method, path, body, timestamp)
}

// DeriveWSAuth generates credentials for authenticated websocket subscriptions.
func (c *AuthenticatedClient) DeriveWSAuth(ctx context.Context) (WSAuth, error) {
	creds := c.credentials()
	if creds == nil {
		return WSAuth{}, fmt.Errorf("derive ws auth requires API credentials")
	}

	timestamp, err := c.timestamp(ctx)
	if err != nil {
		return WSAuth{}, err
	}

	// For the WS user channel, we use GET /ws/user as the signing path
	signature, err := polyauth.HMACSignature(creds.Secret, timestamp, "GET", "/ws/user", nil)
	if err != nil {
		return WSAuth{}, err
	}

	return WSAuth{
		Key:        creds.Key,
		Passphrase: creds.Passphrase,
		Timestamp:  strconv.FormatInt(timestamp, 10),
		Signature:  signature,
	}, nil
}
