package clob

import (
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultHost is the production Polymarket CLOB base URL.
	DefaultHost = "https://clob.polymarket.com"
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
	Host string
	// GeoblockHost overrides the host used for geoblock checks.
	GeoblockHost  string
	ChainID       int64
	PrivateKey    string
	Credentials   *Credentials
	BuilderAuth   BuilderAuth
	SignatureType SignatureType
	FunderAddress string
	HTTPClient    *http.Client
	UserAgent     string
	UseServerTime bool
}

func (c Config) normalized() Config {
	if c.Host == "" {
		c.Host = DefaultHost
	}
	c.Host = strings.TrimRight(c.Host, "/")

	if c.GeoblockHost == "" {
		c.GeoblockHost = DefaultGeoblockHost
	}
	c.GeoblockHost = strings.TrimRight(c.GeoblockHost, "/")

	if c.ChainID == 0 {
		c.ChainID = PolygonChainID
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}

	if c.UserAgent == "" {
		c.UserAgent = defaultUA
	}

	return c
}
