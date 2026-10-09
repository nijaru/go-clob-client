// A fully local combo RFQ demonstration: no live account or trade is used.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// These are native v2 position IDs, not legacy CTF token IDs.
	yes := "1356938545749799165119972480570561420155507632800475359837393562592731987968"
	no := "1356938545749799165119972480570561420155507632800475359837393562592731987969"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws/rfq" {
			_ = json.NewEncoder(w).
				Encode(map[string]string{"rfq_id": "local", "status": "CONFIRMED", "tx_hash": "0x" + strings.Repeat("ab", 32)})
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		send := func(value any) {
			data, _ := json.Marshal(value)
			_ = conn.Write(r.Context(), websocket.MessageText, data)
		}
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		} // Local auth fixture.
		send(map[string]any{"type": "auth", "success": true})
		request := map[string]any{
			"type":                "RFQ_REQUEST",
			"rfq_id":              "local",
			"requestor_public_id": "local-requester",
			"leg_position_ids":    []string{yes, no},
			"condition_id":        "0x0300000000000000000000000000000000000000000000000000000000000000",
			"yes_position_id":     yes,
			"no_position_id":      no,
			"direction":           "BUY",
			"side":                "YES",
			"requested_size":      map[string]string{"unit": "shares", "value_e6": "1000000"},
			"submission_deadline": time.Now().Add(time.Second).UnixMilli(),
		}
		send(request)
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		} // Signed quote.
		send(
			map[string]string{
				"type":     "ACK_RFQ_QUOTE",
				"rfq_id":   "local",
				"quote_id": "local-quote",
			},
		)
		// Demonstrate last look as an explicit caller decision, never automatic.
		request["type"], request["quote_id"], request["signer_address"], request["maker_address"], request["signature_type"], request["fill_size_e6"], request["price_e6"], request["confirm_by"] = "RFQ_CONFIRMATION_REQUEST", "local-quote", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", 0, "1000000", "450000", time.Now().
			Add(time.Second).
			UnixMilli()
		send(request)
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		send(
			map[string]string{
				"type":     "ACK_RFQ_CONFIRMATION_RESPONSE",
				"rfq_id":   "local",
				"quote_id": "local-quote",
				"decision": "DECLINE",
			},
		)
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	client, err := clob.NewAuthenticatedClient(clob.Config{
		ChainID: clob.PolygonChainID,
		// Public development-only key. Never fund this account.
		PrivateKey: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		Credentials: &clob.Credentials{
			Key:        "local",
			Secret:     "c2VjcmV0",
			Passphrase: "local",
		},
		BuilderGatewayHost: server.URL,
	})
	if err != nil {
		log.Fatal(err)
	}
	session, err := client.OpenComboRFQSession(
		ctx,
		clob.ComboRFQSessionOptions{URL: "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/rfq"},
	)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()
	select {
	case event := <-session.Events():
		request, ok := event.(clob.ComboRFQQuoteRequest)
		if !ok {
			log.Fatal("expected quote request")
		}
		quote, err := session.Quote(
			ctx,
			request,
			clob.ComboRFQQuoteResponse{Price: "0.45", Source: clob.ComboQuoteCollateral},
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("quoted", quote.RFQID, quote.QuoteID)
	case <-ctx.Done():
		log.Fatal(ctx.Err())
	}
	select {
	case event := <-session.Events():
		request, ok := event.(clob.ComboRFQConfirmationRequest)
		if !ok {
			log.Fatal("expected last look")
		}
		if _, err := session.RespondToConfirmation(ctx, request.ComboRFQQuoteReference, clob.ComboRFQDecline); err != nil {
			log.Fatal(err)
		}
		fmt.Println("declined last look")
	case <-ctx.Done():
		log.Fatal(ctx.Err())
	}
	// Separately demonstrate requester settlement polling against a local fixture.
	fill, err := client.WaitForComboFill(
		ctx,
		clob.WaitForComboFillParams{RFQID: "local-settlement"},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("settlement fixture", fill.Status, fill.TxHash)
}
