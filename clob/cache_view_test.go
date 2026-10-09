package clob

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFeeCacheSetterWinsOverPendingRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, `{"base_fee":3}`)
	}))
	defer server.Close()
	client, err := NewClient(Config{Host: server.URL, RateLimit: -1})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, err := client.GetFeeRate(t.Context(), "1"); finished <- err }()
	<-started
	client.SetFeeRateBPS("1", 10)
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	fee, err := client.GetFeeRate(t.Context(), "1")
	if err != nil || fee.BaseFee != 10 {
		t.Fatalf("late fetch replaced explicit fee: %+v %v", fee, err)
	}
	fee.BaseFee = 20
	view := client.AsPublic()
	fee, err = view.GetFeeRate(t.Context(), "1")
	if err != nil || fee.BaseFee != 10 {
		t.Fatalf("caller mutated shared cache: %+v %v", fee, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := view.GetFeeRate(ctx, "1"); err != context.Canceled {
		t.Fatalf("cache ignored cancellation: %v", err)
	}
}

func TestDeauthenticateJoinsHeartbeatAndDetachesAuthView(t *testing.T) {
	client, err := NewAuthenticatedClient(
		Config{
			PrivateKey:  "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
			Credentials: &Credentials{Key: "key", Secret: "c2VjcmV0", Passphrase: "pass"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.StartHeartbeats(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err := client.Deauthenticate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if client.HeartbeatsActive() || view.http.Headers != nil || view.gatewayHTTP.Headers != nil {
		t.Fatal("deauthentication retained lifecycle/auth resolver")
	}
	if client.http.Headers == nil || client.Credentials() == nil {
		t.Fatal("deauthentication unexpectedly invalidated other owned references")
	}
	view.SetFeeRate("1", FeeRateResponse{BaseFee: 7})
	fee, err := client.GetFeeRate(t.Context(), "1")
	if err != nil || fee.BaseFee != 7 {
		t.Fatalf("public view lost shared cache: %v %v", fee, err)
	}
}
