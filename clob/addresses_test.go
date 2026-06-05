package clob

import "testing"

func TestContractAddressHelpers(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{ChainID: PolygonChainID})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	collateral, err := client.GetCollateralAddress()
	if err != nil {
		t.Fatalf("get collateral address: %v", err)
	}
	if collateral != "0xC011a7E12a19f7B1f670d46F03B03f3342E82DFB" {
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
	if exchange != "0xE111180000d2663C0091e4f400237545B87B996B" {
		t.Fatalf("unexpected exchange address: %s", exchange)
	}

	negRiskExchange, err := client.GetExchangeAddress(true)
	if err != nil {
		t.Fatalf("get neg-risk exchange address: %v", err)
	}
	if negRiskExchange != "0xe2222d279d744050d28e00520010520000310F59" {
		t.Fatalf("unexpected neg-risk exchange address: %s", negRiskExchange)
	}
}

func TestContractAddressHelpersUnsupportedChain(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{ChainID: 1})
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
