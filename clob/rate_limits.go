package clob

import (
	"fmt"
	"math"

	"golang.org/x/time/rate"
)

// RateBudget is an optional client-side rate budget. Zero PerSecond disables
// client-side throttling; the exchange still enforces its own limits.
type RateBudget struct {
	PerSecond float64
	Burst     int
}

// RateLimits keeps cancellations independent of regular order submissions
// and market-data reads so they cannot exhaust the same local budget.
type RateLimits struct {
	Reads   RateBudget
	Orders  RateBudget
	Cancels    RateBudget
	Heartbeats RateBudget
	Other      RateBudget
}

type requestLimiters struct {
	reads   *rate.Limiter
	orders  *rate.Limiter
	cancels    *rate.Limiter
	heartbeats *rate.Limiter
	other      *rate.Limiter
}

func (r RateLimits) validate() error {
	for name, budget := range map[string]RateBudget{
		"reads": r.Reads, "orders": r.Orders,
		"cancels": r.Cancels, "heartbeats": r.Heartbeats, "other": r.Other,
	} {
		if math.IsNaN(budget.PerSecond) || math.IsInf(budget.PerSecond, 0) ||
			budget.PerSecond < 0 || budget.Burst < 0 {
			return fmt.Errorf("invalid %s rate budget", name)
		}
	}
	return nil
}

func (r RateLimits) limiters() requestLimiters {
	return requestLimiters{
		reads:   r.Reads.newLimiter(),
		orders:  r.Orders.newLimiter(),
		cancels:    r.Cancels.newLimiter(),
		heartbeats: r.Heartbeats.newLimiter(),
		other:      r.Other.newLimiter(),
	}
}

func (b RateBudget) newLimiter() *rate.Limiter {
	if b.PerSecond == 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(b.PerSecond), max(1, b.Burst))
}
