package clob

import (
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultHost is the production Polymarket CLOB base URL.
	DefaultHost = "https://clob.polymarket.com"
	// PolygonChainID is the Polygon mainnet chain ID used by Polymarket.
	PolygonChainID = int64(137)
	defaultUA      = "go-clob-client/clob"
)

// SignatureType controls which signer/funder model Polymarket should expect for the account.
type SignatureType int

const (
	SignatureTypeEOA SignatureType = iota
	SignatureTypePolyProxy
	SignatureTypePolyGnosisSafe

	SignatureTypeMagic        = SignatureTypePolyProxy
	SignatureTypeBrowserProxy = SignatureTypePolyGnosisSafe
)

// Config configures a Polymarket CLOB client.
type Config struct {
	Host          string
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
