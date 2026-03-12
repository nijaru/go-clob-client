package clob

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalBuilderAuthHeaders(t *testing.T) {
	t.Parallel()

	auth := NewLocalBuilderAuth(Credentials{
		Key:        "builder-key",
		Secret:     "c2VjcmV0",
		Passphrase: "builder-pass",
	})

	headers, err := auth.Headers(context.Background(), BuilderHeaderRequest{
		Method:    http.MethodPost,
		Path:      postOrderEndpoint,
		Body:      []byte(`{"order":"payload"}`),
		Timestamp: 1710000000,
	})
	if err != nil {
		t.Fatalf("builder headers: %v", err)
	}

	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte("1710000000POST/order{\"order\":\"payload\"}"))
	expectedSignature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if headers["POLY_BUILDER_API_KEY"] != "builder-key" {
		t.Fatalf("unexpected builder key header: %q", headers["POLY_BUILDER_API_KEY"])
	}
	if headers["POLY_BUILDER_PASSPHRASE"] != "builder-pass" {
		t.Fatalf(
			"unexpected builder passphrase header: %q",
			headers["POLY_BUILDER_PASSPHRASE"],
		)
	}
	if headers["POLY_BUILDER_TIMESTAMP"] != "1710000000" {
		t.Fatalf("unexpected builder timestamp: %q", headers["POLY_BUILDER_TIMESTAMP"])
	}
	if headers["POLY_BUILDER_SIGNATURE"] != expectedSignature {
		t.Fatalf("unexpected builder signature: %q", headers["POLY_BUILDER_SIGNATURE"])
	}
}

func TestRemoteBuilderAuthHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token-123" {
			t.Fatalf("unexpected auth header: %q", auth)
		}

		var payload struct {
			Method    string `json:"method"`
			Path      string `json:"path"`
			Body      string `json:"body"`
			Timestamp int64  `json:"timestamp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Method != http.MethodGet || payload.Path != builderTradesEndpoint {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		if payload.Body != "" || payload.Timestamp != 1710000000 {
			t.Fatalf("unexpected remote builder payload: %+v", payload)
		}

		_, _ = w.Write([]byte(`{
			"poly_builder_api_key":"remote-builder",
			"poly_builder_timestamp":"1710000000",
			"poly_builder_passphrase":"remote-pass",
			"poly_builder_signature":"remote-sig"
		}`))
	}))
	defer server.Close()

	auth, err := NewRemoteBuilderAuth(RemoteBuilderAuthConfig{
		URL:         server.URL,
		BearerToken: "token-123",
	})
	if err != nil {
		t.Fatalf("new remote builder auth: %v", err)
	}

	headers, err := auth.Headers(context.Background(), BuilderHeaderRequest{
		Method:    http.MethodGet,
		Path:      builderTradesEndpoint,
		Timestamp: 1710000000,
	})
	if err != nil {
		t.Fatalf("builder headers: %v", err)
	}

	if headers["POLY_BUILDER_API_KEY"] != "remote-builder" {
		t.Fatalf("unexpected remote builder key: %q", headers["POLY_BUILDER_API_KEY"])
	}
	if headers["POLY_BUILDER_SIGNATURE"] != "remote-sig" {
		t.Fatalf("unexpected remote builder signature: %q", headers["POLY_BUILDER_SIGNATURE"])
	}
}

func TestNewRemoteBuilderAuthValidation(t *testing.T) {
	t.Parallel()

	if _, err := NewRemoteBuilderAuth(RemoteBuilderAuthConfig{}); err == nil {
		t.Fatal("expected empty URL validation error")
	}
	if _, err := NewRemoteBuilderAuth(RemoteBuilderAuthConfig{URL: "://bad-url"}); err == nil {
		t.Fatal("expected malformed URL validation error")
	}

	auth, err := NewRemoteBuilderAuth(RemoteBuilderAuthConfig{URL: "https://example.com/sign"})
	if err != nil {
		t.Fatalf("new remote builder auth: %v", err)
	}

	remote, ok := auth.(*remoteBuilderAuth)
	if !ok {
		t.Fatalf("expected remote builder auth implementation, got %T", auth)
	}
	if remote.httpClient == nil {
		t.Fatal("expected default HTTP client")
	}
}

func TestBuilderAndHeartbeatEndpoints(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method + " " + r.URL.Path {
		case http.MethodGet + " " + orderEndpoint + "order-1":
			if r.Header.Get("POLY_API_KEY") != "api-key" {
				t.Fatalf("missing L2 api key header")
			}
			if r.Header.Get("POLY_BUILDER_API_KEY") != "builder-key" {
				t.Fatalf("missing builder api key header")
			}
			if r.Header.Get("POLY_TIMESTAMP") != r.Header.Get("POLY_BUILDER_TIMESTAMP") {
				t.Fatalf("expected builder and L2 timestamps to match")
			}
			_ = json.NewEncoder(w).Encode(OpenOrder{ID: "order-1"})
		case http.MethodPost + " " + createBuilderAPIKeyEndpoint:
			if r.Header.Get("POLY_API_KEY") != "api-key" {
				t.Fatalf("missing L2 api key header on create builder key")
			}
			_, _ = w.Write(
				[]byte(`{"apiKey":"builder-key","secret":"c2VjcmV0","passphrase":"builder-pass"}`),
			)
		case http.MethodGet + " " + getBuilderAPIKeysEndpoint:
			if r.Header.Get("POLY_API_KEY") != "api-key" {
				t.Fatalf("missing L2 api key header on get builder keys")
			}
			_, _ = w.Write([]byte(`[{"key":"builder-key","createdAt":"2026-03-12T18:00:00Z"}]`))
		case http.MethodDelete + " " + revokeBuilderAPIKeyEndpoint:
			if r.Header.Get("POLY_BUILDER_API_KEY") != "builder-key" {
				t.Fatalf("missing builder api key header on revoke")
			}
			if r.Header.Get("POLY_API_KEY") != "" {
				t.Fatalf("expected revoke builder key to omit L2 headers")
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodGet + " " + builderTradesEndpoint:
			if r.Header.Get("POLY_BUILDER_API_KEY") != "builder-key" {
				t.Fatalf("missing builder api key header on builder trades")
			}
			if r.Header.Get("POLY_API_KEY") != "" {
				t.Fatalf("expected builder trades to omit L2 headers")
			}
			switch r.URL.Query().Get("next_cursor") {
			case initialCursor:
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "cursor-2",
					"data": [{
						"id":"builder-trade-1",
						"tradeType":"MATCHED",
						"takerOrderHash":"0xabc",
						"builder":"0xbuilder",
						"market":"market-1",
						"assetId":"asset-1",
						"side":"BUY",
						"size":"10",
						"sizeUsdc":"4.2",
						"price":"0.42",
						"status":"MATCHED",
						"outcome":"YES",
						"outcomeIndex":0,
						"requestId":"request-1"
					}]
				}`))
			case "cursor-2":
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "LTE=",
					"data": [{
						"id":"builder-trade-2",
						"tradeType":"MATCHED",
						"takerOrderHash":"0xdef",
						"builder":"0xbuilder",
						"market":"market-2",
						"assetId":"asset-2",
						"side":"SELL",
						"size":"6",
						"sizeUsdc":"3.3",
						"price":"0.55",
						"status":"MATCHED",
						"outcome":"NO",
						"outcomeIndex":1,
						"requestId":"request-2"
					}]
				}`))
			default:
				t.Fatalf("unexpected builder trades cursor: %q", r.URL.Query().Get("next_cursor"))
			}
		case http.MethodPost + " " + heartbeatEndpoint:
			if r.Header.Get("POLY_API_KEY") != "api-key" {
				t.Fatalf("missing L2 api key header on heartbeat")
			}

			var payload map[string]*string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode heartbeat payload: %v", err)
			}
			if payload["heartbeat_id"] == nil {
				_, _ = w.Write([]byte(`{"heartbeat_id":"heartbeat-1"}`))
				return
			}
			if *payload["heartbeat_id"] != "heartbeat-1" {
				t.Fatalf("unexpected heartbeat id: %q", *payload["heartbeat_id"])
			}
			_, _ = w.Write([]byte(`{"heartbeat_id":"heartbeat-2"}`))
		default:
			t.Fatalf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
		BuilderAuth: NewLocalBuilderAuth(Credentials{
			Key:        "builder-key",
			Secret:     "c2VjcmV0",
			Passphrase: "builder-pass",
		}),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.GetOrder(context.Background(), "order-1"); err != nil {
		t.Fatalf("get order with builder headers: %v", err)
	}

	creds, err := client.CreateBuilderAPIKey(context.Background())
	if err != nil {
		t.Fatalf("create builder api key: %v", err)
	}
	if creds.Key != "builder-key" {
		t.Fatalf("unexpected builder creds: %+v", creds)
	}

	keys, err := client.GetBuilderAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("get builder api keys: %v", err)
	}
	if len(keys) != 1 || keys[0].Key != "builder-key" {
		t.Fatalf("unexpected builder api keys: %+v", keys)
	}

	if err := client.RevokeBuilderAPIKey(context.Background()); err != nil {
		t.Fatalf("revoke builder api key: %v", err)
	}

	builderTradesPage, err := client.GetBuilderTradesPage(context.Background(), TradeParams{}, "")
	if err != nil {
		t.Fatalf("get builder trades page: %v", err)
	}
	if len(builderTradesPage.Data) != 1 || builderTradesPage.Data[0].ID != "builder-trade-1" {
		t.Fatalf("unexpected builder trades page: %+v", builderTradesPage)
	}

	builderTrades, err := client.GetBuilderTrades(context.Background(), TradeParams{})
	if err != nil {
		t.Fatalf("get builder trades: %v", err)
	}
	if len(builderTrades) != 2 || builderTrades[1].ID != "builder-trade-2" {
		t.Fatalf("unexpected builder trades: %+v", builderTrades)
	}

	heartbeat, err := client.PostHeartbeat(context.Background(), nil)
	if err != nil {
		t.Fatalf("post heartbeat nil: %v", err)
	}
	if heartbeat.HeartbeatID != "heartbeat-1" {
		t.Fatalf("unexpected initial heartbeat response: %+v", heartbeat)
	}

	nextHeartbeatID := heartbeat.HeartbeatID
	heartbeat, err = client.PostHeartbeat(context.Background(), &nextHeartbeatID)
	if err != nil {
		t.Fatalf("post heartbeat chained: %v", err)
	}
	if heartbeat.HeartbeatID != "heartbeat-2" {
		t.Fatalf("unexpected chained heartbeat response: %+v", heartbeat)
	}
}
