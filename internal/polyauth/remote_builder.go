package polyauth

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	json "github.com/go-json-experiment/json"
)

type RemoteBuilderHeaderRequest struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

type RemoteBuilderHeaderResponse struct {
	APIKey     string `json:"poly_builder_api_key"`
	Timestamp  string `json:"poly_builder_timestamp"`
	Passphrase string `json:"poly_builder_passphrase"`
	Signature  string `json:"poly_builder_signature"`
}

func FetchRemoteBuilderHeaders(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	bearerToken string,
	request RemoteBuilderHeaderRequest,
) (map[string]string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal remote builder request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create remote builder request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform remote builder request: %w", err)
	}
	defer resp.Body.Close()

	const maxResponseBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read remote builder response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("remote builder response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"remote builder signer returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var decoded RemoteBuilderHeaderResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode remote builder response: %w", err)
	}

	if decoded.APIKey == "" || decoded.Timestamp == "" || decoded.Passphrase == "" ||
		decoded.Signature == "" {
		return nil, fmt.Errorf("remote builder response has incomplete authentication headers")
	}
	return map[string]string{
		"POLY_BUILDER_API_KEY":    decoded.APIKey,
		"POLY_BUILDER_SIGNATURE":  decoded.Signature,
		"POLY_BUILDER_TIMESTAMP":  decoded.Timestamp,
		"POLY_BUILDER_PASSPHRASE": decoded.Passphrase,
	}, nil
}
