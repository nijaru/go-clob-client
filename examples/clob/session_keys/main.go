// List Deposit Wallet session keys. Mutations require explicit
// POLYMARKET_SESSION_ACTION=authorize|revoke and POLYMARKET_SESSION_ADDRESS.
// The application generates/stores the session private key; the SDK only needs
// its address. Authorization defaults to ALL and expires after 4,315 hours.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	var builder clob.BuilderAuth
	var err error
	if os.Getenv("POLYMARKET_SESSION_ACTION") != "" {
		builder, err = clob.NewLocalBuilderAuth(
			clob.Credentials{
				Key:        os.Getenv("POLYMARKET_BUILDER_KEY"),
				Secret:     os.Getenv("POLYMARKET_BUILDER_SECRET"),
				Passphrase: os.Getenv("POLYMARKET_BUILDER_PASSPHRASE"),
			},
		)
		if err != nil {
			log.Fatal(err)
		}
	}
	client, err := clob.NewAuthenticatedClient(clob.Config{
		ChainID: clob.PolygonChainID, PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"), SignatureType: clob.SignatureTypePoly1271, FunderAddress: os.Getenv("POLYMARKET_FUNDER"), BuilderAuth: builder,
		Credentials: &clob.Credentials{
			Key:        os.Getenv("POLYMARKET_API_KEY"),
			Secret:     os.Getenv("POLYMARKET_API_SECRET"),
			Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if action := os.Getenv("POLYMARKET_SESSION_ACTION"); action != "" {
		address := os.Getenv("POLYMARKET_SESSION_ADDRESS")
		if !common.IsHexAddress(address) {
			log.Fatal("invalid session address")
		}
		idempotency := os.Getenv("POLYMARKET_IDEMPOTENCY_KEY")
		switch action {
		case "authorize":
			var scopes []clob.SessionKeyScope
			if raw := os.Getenv("POLYMARKET_SESSION_SCOPES"); raw != "" {
				for _, scope := range strings.Split(raw, ",") {
					scopes = append(scopes, clob.SessionKeyScope(strings.TrimSpace(scope)))
				}
			}
			result, err := client.AuthorizeSessionKey(
				ctx,
				clob.AuthorizeSessionKeyRequest{
					Address:        common.HexToAddress(address),
					Scopes:         scopes,
					IdempotencyKey: idempotency,
				},
			)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf(
				"Authorized %s until %d, tx=%s\n",
				result.SessionKey.Address,
				result.SessionKey.ValidUntil,
				result.Transaction.TransactionHash,
			)
		case "revoke":
			if err := client.RevokeSessionKey(ctx, clob.RevokeSessionKeyRequest{Address: common.HexToAddress(address), IdempotencyKey: idempotency}); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Removed from active registry; on-chain revocation may still be pending")
		default:
			log.Fatal("unknown session action")
		}
	}
	keys, err := client.FetchSessionKeys(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, key := range keys {
		fmt.Printf("%s scopes=%v expiry=%d\n", key.Address, key.Scopes, key.ValidUntil)
	}
}
