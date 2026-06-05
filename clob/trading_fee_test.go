package clob

import (
	"testing"

	"github.com/quagmt/udecimal"
)

func TestAdjustMarketBuyAmountBalanceSufficient(t *testing.T) {
	t.Parallel()

	// Balance covers amount + fees → amount unchanged
	amount := udecimal.MustParse("100")
	balance := udecimal.MustParse("200")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, balance, price, 0, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted.Cmp(amount) != 0 {
		t.Errorf("expected %s, got %s", amount, adjusted)
	}
}

func TestAdjustMarketBuyAmountBalanceInsufficient(t *testing.T) {
	t.Parallel()

	// Balance < amount → amount shrinks
	amount := udecimal.MustParse("100")
	balance := udecimal.MustParse("50")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, balance, price, 0, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted.Cmp(amount) >= 0 {
		t.Errorf("expected adjusted < amount, got %s >= %s", adjusted, amount)
	}
	// With no fees, adjusted should equal balance
	if adjusted.Cmp(balance) != 0 {
		t.Errorf("expected %s, got %s", balance, adjusted)
	}
}

func TestAdjustMarketBuyAmountWithPlatformFee(t *testing.T) {
	t.Parallel()

	// Platform fee: rate=0.02, exponent=1
	// At price=0.5: base = 0.5*0.5 = 0.25, fee_rate = 0.02 * 0.25 = 0.005
	// total_cost = 100 + (100/0.5)*0.005 + 0 = 100 + 1 = 101
	// balance=101 should be exactly enough
	amount := udecimal.MustParse("100")
	balance := udecimal.MustParse("101")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, balance, price, 0.02, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Balance covers total cost → amount unchanged
	if adjusted.Cmp(amount) != 0 {
		t.Errorf("expected %s, got %s", amount, adjusted)
	}

	// Balance=100 < 101 → should shrink
	balance2 := udecimal.MustParse("100")
	adjusted2, err := adjustMarketBuyAmount(amount, balance2, price, 0.02, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted2.Cmp(amount) >= 0 {
		t.Errorf("expected adjusted < amount, got %s >= %s", adjusted2, amount)
	}
}

func TestAdjustMarketBuyAmountWithBuilderFee(t *testing.T) {
	t.Parallel()

	// Builder taker fee: 1% = 0.01
	// total_cost = 100 + 0 + 100*0.01 = 101
	amount := udecimal.MustParse("100")
	balance := udecimal.MustParse("101")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, balance, price, 0, 0, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted.Cmp(amount) != 0 {
		t.Errorf("expected %s, got %s", amount, adjusted)
	}

	// Balance=100 < 101 → should shrink
	balance2 := udecimal.MustParse("100")
	adjusted2, err := adjustMarketBuyAmount(amount, balance2, price, 0, 0, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// expected = 100 / (1 + 0.01) = 99.009900...
	if adjusted2.Cmp(amount) >= 0 {
		t.Errorf("expected adjusted < amount, got %s >= %s", adjusted2, amount)
	}
}

func TestAdjustMarketBuyAmountWithBothFees(t *testing.T) {
	t.Parallel()

	// Platform fee: rate=0.02, exponent=1, price=0.5 → fee_rate=0.005
	// Builder taker: 1% = 0.01
	// total_cost = 100 + (100/0.5)*0.005 + 100*0.01 = 100 + 1 + 1 = 102
	amount := udecimal.MustParse("100")
	balance := udecimal.MustParse("102")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, balance, price, 0.02, 1, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted.Cmp(amount) != 0 {
		t.Errorf("expected %s, got %s", amount, adjusted)
	}

	// Balance=101 < 102 → should shrink
	balance2 := udecimal.MustParse("101")
	adjusted2, err := adjustMarketBuyAmount(amount, balance2, price, 0.02, 1, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted2.Cmp(amount) >= 0 {
		t.Errorf("expected adjusted < amount, got %s >= %s", adjusted2, amount)
	}
}

func TestAdjustMarketBuyAmountTruncatesTo6Decimals(t *testing.T) {
	t.Parallel()

	// Result should be truncated to 6 decimal places (USDC precision)
	amount := udecimal.MustParse("100.123456789")
	balance := udecimal.MustParse("200")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, balance, price, 0, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be truncated to 6 decimals
	if adjusted.String() != "100.123456" {
		t.Errorf("expected 100.123456, got %s", adjusted)
	}
}

func TestAdjustMarketBuyAmountZeroBalanceErrors(t *testing.T) {
	t.Parallel()

	amount := udecimal.MustParse("100")
	balance := udecimal.Zero
	price := udecimal.MustParse("0.5")

	_, err := adjustMarketBuyAmount(amount, balance, price, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error for zero balance")
	}
}

func TestAdjustMarketBuyAmountMaxSpendSemantics(t *testing.T) {
	t.Parallel()

	// maxSpend=10 means "spend at most $10 including fees"
	// With no fees: adjusted = 10
	amount := udecimal.MustParse("100")
	maxSpend := udecimal.MustParse("10")
	price := udecimal.MustParse("0.5")

	adjusted, err := adjustMarketBuyAmount(amount, maxSpend, price, 0, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted.Cmp(maxSpend) != 0 {
		t.Errorf("expected %s, got %s", maxSpend, adjusted)
	}

	// With 1% builder fee: adjusted = 10 / (1 + 0.01) = 9.900990...
	adjusted2, err := adjustMarketBuyAmount(amount, maxSpend, price, 0, 0, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adjusted2.Cmp(maxSpend) >= 0 {
		t.Errorf("expected adjusted < maxSpend, got %s >= %s", adjusted2, maxSpend)
	}
}
