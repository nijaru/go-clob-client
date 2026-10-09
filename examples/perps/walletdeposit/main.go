// Wallet-managed perps collateral. Preparation is offline by default; every
// mutation requires an explicit -action. Check chain, contracts and base units
// before sending. Never use the README's fixture key for a funded wallet.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/perps"
	"github.com/nijaru/go-clob-client/signing"
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
		"prepare, discover-deposit, approve, deposit, approve-deposit, or deploy",
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
	// Replace this local convenience with a hardware/remote signing.Signer.
	// EOA execution needs signing.TransactionSigner or TransactionSender.
	// A provider may also implement TransactionWaiter for verified replacements.
	signer, err := signing.NewLocalSigner(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if *action == "discover-deposit" {
		client, err := clob.NewClient(
			clob.Config{ChainID: *chain, RelayerHost: os.Getenv("RELAYER_HOST")},
		)
		if err != nil {
			return err
		}
		wallet, err := client.DiscoverDepositWallet(ctx, signer.Address())
		if err != nil {
			return err
		}
		fmt.Printf(
			"read-only legacy-first discovery: WALLET_ADDRESS=%s; no mutation performed\n",
			wallet,
		)
		return nil
	}
	walletAddress := os.Getenv("WALLET_ADDRESS")
	if walletAddress == "" {
		var derived common.Address
		switch signature {
		case clob.SignatureTypeEOA:
			derived = signer.Address()
		case clob.SignatureTypePolyGnosisSafe:
			derived, err = clob.DeriveSafeWallet(signer.Address(), *chain)
		case clob.SignatureTypePoly1271:
			derived, err = clob.DeriveBeaconDepositWallet(signer.Address(), *chain)
		}
		if err != nil {
			return err
		}
		walletAddress = derived.Hex()
	}
	ownerConfig := perps.OwnerConfig{
		Config: perps.Config{ChainID: *chain}, Signer: signer, Wallet: walletAddress,
		CollateralToken: os.Getenv(
			"COLLATERAL_TOKEN",
		), DepositContract: os.Getenv("PERPS_DEPOSIT_CONTRACT"),
	}
	fmt.Printf(
		"chain=%d wallet=%s credit-owner=%s amount=%s\n",
		*chain,
		walletAddress,
		signer.Address(),
		amount,
	)
	if *action == "prepare" {
		// Preparation needs neither RPC, BuilderAuth nor CLOB L2 credentials.
		owner, err := perps.NewOwner(ownerConfig)
		if err != nil {
			return err
		}
		call, err := owner.PrepareDeposit(amount)
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
	switch *action {
	case "approve", "deposit", "approve-deposit":
	case "deploy":
		if *kind == "eoa" {
			return fmt.Errorf("EOAs do not require deployment")
		}
	default:
		return fmt.Errorf("unknown action %q", *action)
	}
	var builder clob.BuilderAuth
	if *kind != "eoa" {
		builder, err = clob.NewLocalBuilderAuth(clob.Credentials{
			Key: os.Getenv("BUILDER_API_KEY"), Secret: os.Getenv("BUILDER_API_SECRET"),
			Passphrase: os.Getenv("BUILDER_API_PASSPHRASE"),
		})
		if err != nil {
			return err
		}
	}
	wallet, err := perps.NewCollateralWallet(perps.CollateralWalletConfig{
		Owner: ownerConfig,
		Transactions: clob.Config{
			ChainID: *chain, Signer: signer, SignatureType: signature,
			FunderAddress: walletAddress, RPCURL: os.Getenv("RPC_URL"),
			RelayerHost: os.Getenv("RELAYER_HOST"), BuilderAuth: builder,
		},
	})
	if err != nil {
		return err
	}
	var tx *perps.CollateralTransaction
	switch *action {
	case "approve":
		tx, err = wallet.ApproveCollateral(ctx, amount, "Perps collateral approval")
	case "deposit":
		tx, err = wallet.Deposit(ctx, amount, "Perps collateral deposit")
	case "approve-deposit":
		tx, err = wallet.ApproveAndDeposit(ctx, amount, "Perps collateral approval and deposit")
	case "deploy":
		if *kind == "safe" {
			tx, err = wallet.DeploySafeWallet(ctx, "Deploy perps Safe")
		} else {
			tx, err = wallet.DeployDepositWallet(ctx, "Deploy perps deposit wallet")
		}
	}
	if tx != nil {
		for _, s := range tx.Submissions {
			fmt.Printf(
				"submitted %s id=%s hash=%s uncertain=%t receipt-observed=%t\n",
				s.Operation,
				s.TransactionID,
				s.TransactionHash,
				s.BroadcastUncertain,
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
			"mined receipt hash=%s block=%s status=%d\n",
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
