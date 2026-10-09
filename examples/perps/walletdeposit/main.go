// Wallet-managed perps collateral. Preparation is the default; every mutation
// requires an explicit -action. Never run a send action against a live wallet
// without checking the chain, contracts, wallet and base-unit amount first.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/perps"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	action := flag.String(
		"action",
		"prepare",
		"prepare, approve, deposit, approve-deposit, or deploy",
	)
	amountText := flag.String(
		"amount",
		"1000000",
		"exact collateral base units (not decimal tokens)",
	)
	kind := flag.String("wallet", "eoa", "eoa, safe, or deposit")
	chain := flag.Int64("chain", 137, "EVM chain ID")
	flag.Parse()
	amount, ok := new(big.Int).SetString(*amountText, 10)
	if !ok {
		return fmt.Errorf("invalid base-unit amount")
	}
	var signature clob.SignatureType
	switch *kind {
	case "eoa":
		signature = clob.SignatureTypeEOA
	case "safe":
		signature = clob.SignatureTypePolyGnosisSafe
	case "deposit":
		signature = clob.SignatureTypePoly1271
	default:
		return fmt.Errorf("unknown wallet type %q", *kind)
	}
	key := os.Getenv("PRIVATE_KEY")
	signer, err := perps.NewOwnerSigner(key)
	if err != nil {
		return err
	}
	var builder clob.BuilderAuth
	if *kind != "eoa" {
		builder, err = clob.NewLocalBuilderAuth(
			clob.Credentials{
				Key:        os.Getenv("BUILDER_API_KEY"),
				Secret:     os.Getenv("BUILDER_API_SECRET"),
				Passphrase: os.Getenv("BUILDER_API_PASSPHRASE"),
			},
		)
		if err != nil {
			return err
		}
	}
	wallet, err := perps.NewCollateralWallet(perps.CollateralWalletConfig{
		Owner: perps.OwnerConfig{
			Config:          perps.Config{ChainID: *chain},
			Signer:          signer,
			CollateralToken: os.Getenv("COLLATERAL_TOKEN"),
			DepositContract: os.Getenv("PERPS_DEPOSIT_CONTRACT"),
		},
		Transactions: clob.Config{
			ChainID:       *chain,
			PrivateKey:    key,
			SignatureType: signature,
			FunderAddress: os.Getenv("WALLET_ADDRESS"),
			RPCURL:        os.Getenv("RPC_URL"),
			RelayerHost:   os.Getenv("RELAYER_HOST"),
			BuilderAuth:   builder,
			Credentials: &clob.Credentials{
				Key:        os.Getenv("CLOB_API_KEY"),
				Secret:     os.Getenv("CLOB_API_SECRET"),
				Passphrase: os.Getenv("CLOB_API_PASSPHRASE"),
			},
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf(
		"chain=%d wallet=%s credit-owner=%s amount=%s\n",
		*chain,
		wallet.WalletAddress(),
		signer.Address(),
		amount,
	)
	if *action == "prepare" {
		call, err := wallet.PrepareDeposit(amount)
		if err != nil {
			return err
		}
		fmt.Printf(
			"unsigned deposit to=%s data=0x%x; no RPC or mutation performed\n",
			call.To,
			call.Data,
		)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var tx *perps.CollateralTransaction
	switch *action {
	case "approve":
		tx, err = wallet.ApproveCollateral(ctx, amount, "Perps collateral approval")
	case "deposit":
		tx, err = wallet.Deposit(ctx, amount, "Perps collateral deposit")
	case "approve-deposit":
		tx, err = wallet.ApproveAndDeposit(ctx, amount, "Perps collateral approval and deposit")
	case "deploy":
		tx, err = wallet.DeployDepositWallet(ctx, "Deploy perps deposit wallet")
	default:
		return fmt.Errorf("unknown action %q", *action)
	}
	if tx != nil {
		for _, s := range tx.Submissions {
			fmt.Printf(
				"submitted %s id=%s hash=%s prefix-confirmed=%t\n",
				s.Operation,
				s.TransactionID,
				s.TransactionHash,
				s.ConfirmedReceipt != nil,
			)
		}
	}
	if err != nil {
		return fmt.Errorf("reconcile submissions before retrying: %w", err)
	}
	receipts, err := tx.Wait(ctx)
	for _, receipt := range receipts {
		fmt.Printf(
			"RPC receipt hash=%s block=%s status=%d\n",
			receipt.TxHash,
			receipt.BlockNumber,
			receipt.Status,
		)
	}
	if err != nil {
		return err
	}
	fmt.Println("Mined successfully; check perps deposit history for ledger credit.")
	return nil
}
