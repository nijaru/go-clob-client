package perps

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestCancellationRetryWakeDeadline(t *testing.T) {
	// Virtual time reaches the exact retry deadline when the timer fires. The
	// pre-timer forecast permits this wait, but must not authorize an expired
	// resend on wake. No wall-clock scheduling or request-count timing involved.
	for _, margin := range []time.Duration{0, time.Nanosecond} {
		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			ready, err := waitCancelRetry(t.Context(), 1, start.Add(100*time.Millisecond+margin))
			if err != nil || ready != (margin > 0) || time.Since(start) != 100*time.Millisecond {
				t.Fatalf(
					"margin %v: ready=%v err=%v elapsed=%v",
					margin,
					ready,
					err,
					time.Since(start),
				)
			}
		})
	}
}

func TestCancellationRetryWakeCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		ready, err := waitCancelRetry(ctx, 1, time.Now().Add(time.Second))
		if ready || !errors.Is(err, context.Canceled) {
			t.Fatalf("ready=%v err=%v", ready, err)
		}
	})
}
