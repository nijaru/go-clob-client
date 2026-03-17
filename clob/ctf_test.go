package clob

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyauth"
)

func TestCTFOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	key, _ := polyauth.GenerateKey()
	clientRaw, _ := New(Config{
		Host:       server.URL,
		ChainID:    137,
		PrivateKey: key,
		Credentials: &Credentials{
			Key:        "key",
			Secret:     "secret",
			Passphrase: "pass",
		},
	})
	client := clientRaw.(*AuthenticatedClient)

	ctx := context.Background()
	conditionID := "0x123"
	amount := "100"

	if err := client.SplitTokens(ctx, SplitArgs{ConditionID: conditionID, Amount: amount}); err != nil {
		t.Errorf("SplitTokens failed: %v", err)
	}

	if err := client.MergeTokens(ctx, MergeArgs{ConditionID: conditionID, Amount: amount}); err != nil {
		t.Errorf("MergeTokens failed: %v", err)
	}

	if err := client.RedeemTokens(ctx, RedeemArgs{ConditionID: conditionID}); err != nil {
		t.Errorf("RedeemTokens failed: %v", err)
	}
}
