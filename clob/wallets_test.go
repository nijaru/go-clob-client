package clob_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/clob"
)

// Known test key from Foundry/Anvil: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
var testEOA = common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")

func TestDeriveSafeWalletPolygon(t *testing.T) {
	t.Parallel()
	addr, err := clob.DeriveSafeWallet(testEOA, clob.PolygonChainID)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	want := common.HexToAddress("0xd93b25Cb943D14d0d34FBAf01fc93a0F8b5f6e47")
	if addr != want {
		t.Errorf("safe wallet = %s, want %s", addr.Hex(), want.Hex())
	}
}

func TestDeriveSafeWalletAmoy(t *testing.T) {
	t.Parallel()
	addr, err := clob.DeriveSafeWallet(testEOA, clob.AmoyChainID)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	// Same Safe factory on both networks → same derived address
	want := common.HexToAddress("0xd93b25Cb943D14d0d34FBAf01fc93a0F8b5f6e47")
	if addr != want {
		t.Errorf("safe wallet = %s, want %s", addr.Hex(), want.Hex())
	}
}

func TestDeriveProxyWalletPolygon(t *testing.T) {
	t.Parallel()
	addr, err := clob.DeriveProxyWallet(testEOA, clob.PolygonChainID)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	want := common.HexToAddress("0x365f0cA36ae1F641E02Fe3b7743673DA42A13a70")
	if addr != want {
		t.Errorf("proxy wallet = %s, want %s", addr.Hex(), want.Hex())
	}
}

func TestDeriveProxyWalletAmoyNotSupported(t *testing.T) {
	t.Parallel()
	addr, err := clob.DeriveProxyWallet(testEOA, clob.AmoyChainID)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if addr != (common.Address{}) {
		t.Errorf("expected zero address for unsupported chain, got %s", addr.Hex())
	}
}

func TestDeriveWalletUnsupportedChain(t *testing.T) {
	t.Parallel()
	_, err := clob.DeriveSafeWallet(testEOA, 1)
	if err == nil {
		t.Error("expected error for unsupported chain")
	}
	_, err = clob.DeriveProxyWallet(testEOA, 1)
	if err == nil {
		t.Error("expected error for unsupported chain")
	}
}
