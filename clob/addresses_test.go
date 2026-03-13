package clob

import "testing"

func TestContractAddressHelpers(t *testing.T) {
	t.Parallel()

	client, err := New(Config{ChainID: PolygonChainID})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	collateral, err := client.GetCollateralAddress()
	if err != nil {
		t.Fatalf("get collateral address: %v", err)
	}
	if collateral != "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174" {
		t.Fatalf("unexpected collateral address: %s", collateral)
	}

	conditional, err := client.GetConditionalAddress()
	if err != nil {
		t.Fatalf("get conditional address: %v", err)
	}
	if conditional != "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045" {
		t.Fatalf("unexpected conditional address: %s", conditional)
	}

	exchange, err := client.GetExchangeAddress(false)
	if err != nil {
		t.Fatalf("get exchange address: %v", err)
	}
	if exchange != "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E" {
		t.Fatalf("unexpected exchange address: %s", exchange)
	}

	negRiskExchange, err := client.GetExchangeAddress(true)
	if err != nil {
		t.Fatalf("get neg-risk exchange address: %v", err)
	}
	if negRiskExchange != "0xC5d563A36AE78145C45a50134d48A1215220f80a" {
		t.Fatalf("unexpected neg-risk exchange address: %s", negRiskExchange)
	}
}

func TestContractAddressHelpersUnsupportedChain(t *testing.T) {
	t.Parallel()

	client, err := New(Config{ChainID: 1})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.GetCollateralAddress(); err == nil {
		t.Fatal("expected unsupported chain error")
	}
	if _, err := client.GetConditionalAddress(); err == nil {
		t.Fatal("expected unsupported chain error")
	}
	if _, err := client.GetExchangeAddress(false); err == nil {
		t.Fatal("expected unsupported chain error")
	}
}
