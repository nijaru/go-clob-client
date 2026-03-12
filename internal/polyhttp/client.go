package polyhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type AuthLevel int

const (
	AuthNone AuthLevel = iota
	AuthL1
	AuthL2
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
	StatusCode int
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("polymarket API error: status %d", e.StatusCode)
	}
	return fmt.Sprintf("polymarket API error: status %d: %s", e.StatusCode, e.Message)
}

func (c *Client) GetJSON(
	ctx context.Context,
	path string,
	query url.Values,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, auth, nil, out)
}

func (c *Client) PostJSON(
	ctx context.Context,
	path string,
	body any,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodPost, path, nil, body, auth, nil, out)
}

func (c *Client) DeleteJSON(
	ctx context.Context,
	path string,
	body any,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodDelete, path, nil, body, auth, nil, out)
}

func (c *Client) DeleteJSONQuery(
	ctx context.Context,
	path string,
	query url.Values,
	body any,
	auth AuthLevel,
	out any,
) error {
	return c.doJSON(ctx, http.MethodDelete, path, query, body, auth, nil, out)
}

func (c *Client) GetJSONWithNonce(
	ctx context.Context,
	path string,
	query url.Values,
	auth AuthLevel,
	nonce int64,
	out any,
) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, auth, &nonce, out)
}

func (c *Client) PostJSONWithNonce(
	ctx context.Context,
	path string,
	body any,
	auth AuthLevel,
	nonce int64,
	out any,
) error {
	return c.doJSON(ctx, http.MethodPost, path, nil, body, auth, &nonce, out)
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	auth AuthLevel,
	nonce *int64,
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

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if method == http.MethodGet {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	if c.Headers != nil {
		headers, err := c.Headers(ctx, method, path, requestBody, auth, nonce)
		if err != nil {
			return err
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return newAPIError(resp, payload)
	}

	if out == nil || len(payload) == 0 {
		return nil
	}

	if value, ok := out.(*json.RawMessage); ok {
		*value = append((*value)[:0], payload...)
		return nil
	}

	if value, ok := out.(*int64); ok {
		parsed, err := strconv.ParseInt(strings.TrimSpace(string(payload)), 10, 64)
		if err != nil {
			return fmt.Errorf("decode integer response: %w", err)
		}
		*value = parsed
		return nil
	}

	if value, ok := out.(*string); ok {
		var decoded string
		if err := json.Unmarshal(payload, &decoded); err == nil {
			*value = decoded
			return nil
		}
		*value = strings.TrimSpace(string(payload))
		return nil
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	switch typed := body.(type) {
	case []byte:
		return bytes.Clone(typed), nil
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

func newAPIError(resp *http.Response, body []byte) *APIError {
	err := &APIError{
		StatusCode: resp.StatusCode,
		Body:       bytes.Clone(body),
	}

	var payload struct {
		Error any `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != nil {
		err.Message = fmt.Sprint(payload.Error)
		return err
	}

	if len(body) > 0 {
		err.Message = string(body)
	}

	return err
}
