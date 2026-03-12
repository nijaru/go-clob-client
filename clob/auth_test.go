package clob

import (
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyauth"
)

func TestParsePrivateKey(t *testing.T) {
	t.Parallel()

	signer, err := polyauth.ParsePrivateKey(
		"0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
	)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}

	if signer.Address().Hex() == "" {
		t.Fatal("expected non-empty address")
	}
}
