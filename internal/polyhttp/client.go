package polyhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
}

type APIError struct {
	StatusCode  int
	Message     string
	Body        []byte // Response body
	RequestBody []byte // Original request body
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
		StatusCode:  resp.StatusCode,
		Body:        bytes.Clone(body),
		RequestBody: bytes.Clone(requestBody),
	}
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
