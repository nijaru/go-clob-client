package clob

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestComboQuoterAuthenticationFailureAndConnectionLoss(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"authentication rejected", "peer disconnected", "command canceled"} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan struct{}, 1)
			url := newComboWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
				if mode == "authentication rejected" {
					_ = comboWSWrite(
						ctx,
						conn,
						map[string]any{"type": "auth", "success": false, "error": "denied"},
					)
					_, _, _ = conn.Read(ctx)
					return
				}
				_ = comboWSWrite(ctx, conn, map[string]any{"type": "auth", "success": true})
				if _, err := comboWSRead(ctx, conn); err != nil {
					return
				}
				received <- struct{}{}
				if mode == "peer disconnected" {
					return
				}
				_, _, _ = conn.Read(ctx)
			})
			owner, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			client := newComboTestClient(t, "https://unused.invalid")
			session, err := client.OpenComboRFQSession(owner, ComboRFQSessionOptions{URL: url})
			if mode == "authentication rejected" {
				if err == nil || !strings.Contains(err.Error(), "denied") {
					t.Fatalf("auth rejection: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			commandCtx, commandCancel := context.WithCancel(owner)
			defer commandCancel()
			result := make(chan error, 1)
			go func() {
				_, err := session.CancelQuote(
					commandCtx,
					ComboRFQQuoteReference{RFQID: "r", QuoteID: "q"},
				)
				result <- err
			}()
			<-received
			if mode == "command canceled" {
				commandCancel()
			}
			if err := <-result; err == nil {
				t.Fatal("pending command succeeded after connection loss/cancellation")
			}
			<-session.Done()
			if session.Err() == nil {
				t.Fatal("missing terminal error")
			}
			if owner.Err() != nil {
				t.Fatal("session command canceled its owner")
			}
		})
	}
}

func TestComboQuoterSessionKeyBoundary(t *testing.T) {
	t.Parallel()
	client := newSessionOwner(t, "http://127.0.0.1:1")
	client.funderAddress = "0x1111111111111111111111111111111111111111"
	if _, err := client.OpenComboRFQSession(t.Context(), ComboRFQSessionOptions{URL: "ws://127.0.0.1:1"}); !errors.Is(
		err,
		ErrComboSessionKeyUnsupported,
	) {
		t.Fatalf("session-key websocket: %v", err)
	}
	if _, err := client.buildComboQuoterQuote(t.Context(), comboQuoteRequest(), ComboRFQQuoteResponse{Price: "0.5"}); !errors.Is(
		err,
		ErrComboSessionKeyUnsupported,
	) {
		t.Fatalf("session-key quote: %v", err)
	}
	if _, err := client.WaitForComboFill(t.Context(), WaitForComboFillParams{RFQID: "r"}); !errors.Is(
		err,
		ErrComboSessionKeyUnsupported,
	) {
		t.Fatalf("session-key fill wait: %v", err)
	}
}
