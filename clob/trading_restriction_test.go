package clob

import (
	"errors"
	"net/http"
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestTradingRestrictionConstants(t *testing.T) {
	t.Parallel()

	if TradingRestrictionRestarting != polyhttp.TradingRestrictionRestarting {
		t.Errorf("Restarting = %q", TradingRestrictionRestarting)
	}
	if TradingRestrictionCancelOnly != polyhttp.TradingRestrictionCancelOnly {
		t.Errorf("CancelOnly = %q", TradingRestrictionCancelOnly)
	}
	if TradingRestrictionPostOnly != polyhttp.TradingRestrictionPostOnly {
		t.Errorf("PostOnly = %q", TradingRestrictionPostOnly)
	}
}

func TestIsTradingRestriction(t *testing.T) {
	t.Parallel()

	restarting := polyhttp.TradingRestrictionRestarting
	postOnly := polyhttp.TradingRestrictionPostOnly

	if !IsTradingRestriction(&polyhttp.APIError{
		StatusCode:         http.StatusTooEarly,
		TradingRestriction: &restarting,
	}) {
		t.Fatal("expected IsTradingRestriction for restarting")
	}

	if !IsTradingRestriction(&polyhttp.APIError{
		StatusCode:         http.StatusServiceUnavailable,
		TradingRestriction: &postOnly,
	}) {
		t.Fatal("expected IsTradingRestriction for post_only")
	}

	if IsTradingRestriction(&polyhttp.APIError{StatusCode: http.StatusBadRequest}) {
		t.Fatal("did not expect IsTradingRestriction for plain error")
	}
}

func TestAsTradingRestriction(t *testing.T) {
	t.Parallel()

	restriction := TradingRestrictionCancelOnly
	err := &polyhttp.APIError{
		StatusCode:         http.StatusServiceUnavailable,
		TradingRestriction: &restriction,
	}

	got := AsTradingRestriction(err)
	if got == nil || *got != TradingRestrictionCancelOnly {
		t.Fatalf("AsTradingRestriction = %#v, want %q", got, TradingRestrictionCancelOnly)
	}

	if AsTradingRestriction(nil) != nil {
		t.Fatal("expected nil for nil error")
	}

	plain := errors.New("plain")
	if AsTradingRestriction(plain) != nil {
		t.Fatal("expected nil for non-APIError")
	}
}