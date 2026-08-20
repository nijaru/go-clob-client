package polyhttp

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

type AuthLevel int

const (
	AuthNone      AuthLevel = iota
	AuthL1                  // L1: Ethereum-signed timestamp header
	AuthL2                  // L2: HMAC-signed API key headers
	AuthL2Builder           // L2 + builder headers for supported endpoints
)

type HeaderFunc func(
	ctx context.Context,
	method, path string,
	body []byte,
	level AuthLevel,
	nonce *int64,
) (map[string]string, error)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	Headers    HeaderFunc
	// OnRateLimitUpdate is invoked with the parsed Poly-RateLimit-* state
	// whenever a response reports it, both successful and failed. Errors
	// raised by the listener are ignored and must not affect request
	// handling.
	OnRateLimitUpdate func(*RateLimitUpdate)
}

// RateLimitUpdate is the per-signer rate-limit state reported by a response.
// Fields mirror the Poly-RateLimit-* response headers. Each optional field
// is populated independently, so any subset can be present depending on how
// the request was evaluated.
type RateLimitUpdate struct {
	// Remaining is the token balance left in the applicable rate-limit bucket
	// after the request was accounted for. It can be negative for tiers that
	// allow a negative cancellation balance.
	Remaining *float64
	// Reset is the Unix timestamp, in seconds, when the current rate-limit
	// wait period ends.
	Reset *float64
	// Tier is the rate-limit tier applied to the request.
	Tier *string
	// Warning is true when the limiter runs in warning mode and the request
	// would have been rejected under live enforcement.
	Warning bool
}

// TradingRestriction identifies the trading restriction that caused a
// rejection. It mirrors the upstream TS/Python classification.
type TradingRestriction string

const (
	// TradingRestrictionRestarting means the matching engine is restarting
	// and rejects order requests until it is back.
	TradingRestrictionRestarting TradingRestriction = "restarting"
	// TradingRestrictionCancelOnly means cancels are accepted but new orders
	// are rejected.
	TradingRestrictionCancelOnly TradingRestriction = "cancel_only"
	// TradingRestrictionPostOnly means cancels and post-only orders are
	// accepted while other orders are rejected.
	TradingRestrictionPostOnly TradingRestriction = "post_only"
)

type APIError struct {
	StatusCode  int
	Message     string
	Body        []byte // Response body
	RequestBody []byte // Original request body
	// RetryAfterSeconds is the server-requested delay before retrying. It is
	// nil when neither the Retry-After header nor the JSON fallback is valid.
	RetryAfterSeconds *float64
	// Code is the machine-readable error code from the response body, when
	// present. It is nil when the response does not provide one.
	Code *string
	// TradingRestriction identifies the trading restriction that caused the
	// rejection. It is nil when the rejection is not a trading restriction.
	TradingRestriction *TradingRestriction
	// RateLimit is the per-signer rate-limit state reported with the
	// response. It is nil when the response does not report it.
	RateLimit *RateLimitUpdate
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("polymarket API error: status %d", e.StatusCode)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// HTTPStatus returns the HTTP status code of the error.
// It is used by errors.Is matching in the clob package via the HTTPStatuser interface.
func (e *APIError) HTTPStatus() int { return e.StatusCode }

func (e *APIError) Is(target error) bool {
	type HTTPStatuser interface{ HTTPStatus() int }
	if ts, ok := target.(HTTPStatuser); ok {
		return e.StatusCode == ts.HTTPStatus()
	}
	return false
}

func (c *Client) GetJSON(
	ctx context.Context,
	path string,
	query url.Values,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, auth, nil, nil, out)
}

func (c *Client) PostJSON(
	ctx context.Context,
	path string,
	body any,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodPost, path, nil, body, auth, nil, nil, out)
}

func (c *Client) DeleteJSON(
	ctx context.Context,
	path string,
	body any,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, body, auth, nil, nil, out)
}

func (c *Client) DeleteJSONQuery(
	ctx context.Context,
	path string,
	query url.Values,
	body any,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodDelete, path, query, body, auth, nil, nil, out)
}

func (c *Client) GetJSONWithNonce(
	ctx context.Context,
	path string,
	query url.Values,
	auth AuthLevel,
	nonce int64,
	out any,
) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, auth, &nonce, nil, out)
}

func (c *Client) PostJSONWithNonce(
	ctx context.Context,
	path string,
	body any,
	auth AuthLevel,
	nonce int64,
	out any,
) error {
	return c.doJSON(ctx, http.MethodPost, path, nil, body, auth, &nonce, nil, out)
}

func (c *Client) DoJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	auth AuthLevel,
	nonce *int64,
	extraHeaders map[string]string,
	out any,
) error {
	return c.doJSON(ctx, method, path, query, body, auth, nonce, extraHeaders, out)
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	auth AuthLevel,
	nonce *int64,
	extraHeaders map[string]string,
	out any,
) error {
	requestBody, err := marshalBody(body)
	if err != nil {
		return err
	}

	fullURL := c.BaseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if len(requestBody) > 0 {
		bodyReader = bytes.NewReader(requestBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	if c.Headers != nil {
		headers, err := c.Headers(ctx, method, path, requestBody, auth, nonce)
		if err != nil {
			return err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	c.notifyRateLimitUpdate(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(resp.Body)
		return newAPIError(resp, payload, requestBody)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	// Optimization: For certain types, we still need the full payload or special handling
	switch target := out.(type) {
	case *jsontext.Value:
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		*target = append((*target)[:0], payload...)
		return nil
	case *int64:
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		parsed, err := strconv.ParseInt(strings.TrimSpace(string(payload)), 10, 64)
		if err != nil {
			return fmt.Errorf("decode integer response: %w", err)
		}
		*target = parsed
		return nil
	case *string:
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		var decoded string
		if err := json.Unmarshal(payload, &decoded); err == nil {
			*target = decoded
		} else {
			*target = strings.TrimSpace(string(payload))
		}
		return nil
	case *[]byte:
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		*target = payload
		return nil
	default:
		// For everything else, use streaming decode
		if err := json.UnmarshalRead(resp.Body, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
}

// notifyRateLimitUpdate parses the Poly-RateLimit-* headers on every
// response and invokes the configured listener when present. A listener
// error is swallowed so it cannot affect request handling.
func (c *Client) notifyRateLimitUpdate(resp *http.Response) {
	if c.OnRateLimitUpdate == nil {
		return
	}
	update := parseRateLimitHeaders(resp.Header)
	if update == nil {
		return
	}
	func() {
		defer func() {
			recover()
		}()
		c.OnRateLimitUpdate(update)
	}()
}

// parseRateLimitHeaders extracts the Poly-RateLimit-* response headers.
// Returns nil when the response carries none of them.
func parseRateLimitHeaders(headers http.Header) *RateLimitUpdate {
	remaining := parseRateLimitNumber(headers.Get("Poly-RateLimit-Remaining"))
	reset := parseRateLimitNumber(headers.Get("Poly-RateLimit-Reset"))
	tier := parseRateLimitText(headers.Get("Poly-RateLimit-Tier"))
	warningHeader := parseRateLimitText(headers.Get("Poly-RateLimit-Warning"))
	warning := warningHeader != nil && strings.EqualFold(*warningHeader, "true")

	if remaining == nil && reset == nil && tier == nil && !warning {
		return nil
	}

	return &RateLimitUpdate{
		Remaining: remaining,
		Reset:     reset,
		Tier:      tier,
		Warning:   warning,
	}
}

func parseRateLimitNumber(value string) *float64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return nil
	}
	return &parsed
}

func parseRateLimitText(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	out := strings.TrimSpace(value)
	return &out
}

// extractResponseErrorCode pulls the top-level "code" field from a JSON error
// body. It is nil when the body is not JSON, the field is absent, or the value
// is not a non-empty string.
func extractResponseErrorCode(body []byte) *string {
	var payload struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	if payload.Code == "" {
		return nil
	}
	return &payload.Code
}

// detectTradingRestriction classifies a non-success response as a trading
// restriction when the server reports one. It mirrors the upstream TS/Python
// classification:
//
//   - HTTP 425 → restarting
//   - HTTP 503 with body code "post_only_mode" → post_only
//   - HTTP 503 whose message contains "cancel-only" → cancel_only
//
// All other responses return nil.
func detectTradingRestriction(resp *http.Response, body []byte) *TradingRestriction {
	if resp.StatusCode == http.StatusTooEarly {
		restriction := TradingRestrictionRestarting
		return &restriction
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil
	}

	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Error   any    `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}

	if payload.Code == "post_only_mode" {
		restriction := TradingRestrictionPostOnly
		return &restriction
	}

	if msg, ok := payload.Error.(string); ok &&
		strings.Contains(strings.ToLower(msg), "cancel-only") {
		restriction := TradingRestrictionCancelOnly
		return &restriction
	}
	if strings.Contains(strings.ToLower(payload.Message), "cancel-only") {
		restriction := TradingRestrictionCancelOnly
		return &restriction
	}

	return nil
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	switch typed := body.(type) {
	case []byte:
		// Slice is only read downstream, no clone needed.
		return typed, nil
	case string:
		return []byte(typed), nil
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		return payload, nil
	}
}

func newAPIError(resp *http.Response, body, requestBody []byte) *APIError {
	err := &APIError{
		StatusCode:         resp.StatusCode,
		Body:               bytes.Clone(body),
		RequestBody:        bytes.Clone(requestBody),
		RetryAfterSeconds:  retryAfterSeconds(resp.Header.Get("Retry-After"), body),
		RateLimit:          parseRateLimitHeaders(resp.Header),
		TradingRestriction: detectTradingRestriction(resp, body),
	}
	err.Code = extractResponseErrorCode(body)

	var payload struct {
		Error any `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != nil {
		switch e := payload.Error.(type) {
		case string:
			err.Message = e
		case map[string]any:
			if msg, ok := e["message"].(string); ok {
				err.Message = msg
			} else {
				err.Message = fmt.Sprint(e)
			}
		default:
			err.Message = fmt.Sprint(e)
		}
		return err
	}

	if len(body) > 0 {
		err.Message = string(body)
	}

	return err
}

// retryAfterSeconds extracts a finite, non-negative retry delay. A valid
// Retry-After header takes precedence over the JSON body fallback.
func retryAfterSeconds(header string, body []byte) *float64 {
	if seconds, ok := parseRetryAfterNumber(header); ok {
		return &seconds
	}

	var payload struct {
		RetryAfterSeconds stdjson.RawMessage `json:"retry_after_seconds"`
	}
	if stdjson.Unmarshal(body, &payload) != nil {
		return nil
	}
	seconds, ok := parseRetryAfterJSONNumber(payload.RetryAfterSeconds)
	if !ok {
		return nil
	}
	return &seconds
}

func parseRetryAfterNumber(value string) (float64, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return seconds, err == nil && !math.IsInf(seconds, 0) && !math.IsNaN(seconds) && seconds >= 0
}

func parseRetryAfterJSONNumber(raw stdjson.RawMessage) (float64, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, false
	}
	if raw[0] == '"' {
		return 0, false
	}
	return parseRetryAfterNumber(string(raw))
}
