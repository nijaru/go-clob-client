// This example uses a trusted external wallet backend without a private key or
// CLOB API credentials. Its illustrative HTTPS protocol is NOT a Polymarket API:
// POST /sign-typed-data returns {"signature":"0x..."}; POST /send-transaction
// returns {"hash":"0x...","error":"optional failure"}. The backend owns nonce,
// gas, request authorization, signing and broadcast. A hash is opaque: the SDK
// cannot verify its signed intent. Use an authenticated backend you control.
//
// Set WALLET_URL, WALLET_TOKEN and WALLET_ADDRESS. By default this only constructs
// the client. -send explicitly sends TOKEN_ADDRESS / RECIPIENT_ADDRESS / AMOUNT
// (base units) on Polygon and waits using RPC_URL. Never retry an uncertain send
// without reconciliation; an empty hash requires asking the wallet for its hash.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/signing"
)

type remoteWallet struct {
	address common.Address
	url     string
	token   string
	http    *http.Client
}

func (w *remoteWallet) Address() common.Address { return w.address }

func (w *remoteWallet) post(ctx context.Context, path string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.token)
	res, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// Decode even an error response so a known broadcast hash is not discarded.
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(output); err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("wallet HTTP status %d", res.StatusCode)
	}
	return nil
}

func (w *remoteWallet) SignTypedData(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
	var response struct {
		Signature string `json:"signature"`
	}
	if err := w.post(ctx, "/sign-typed-data", data, &response); err != nil {
		return nil, err
	}
	return hexutil.Decode(response.Signature)
}

func (w *remoteWallet) SendTransaction(
	ctx context.Context,
	req signing.TransactionRequest,
) (common.Hash, error) {
	var response struct {
		Hash  common.Hash `json:"hash"`
		Error string      `json:"error"`
	}
	input := struct {
		ChainID string         `json:"chainId"`
		From    common.Address `json:"from"`
		To      common.Address `json:"to"`
		Value   string         `json:"value"`
		Data    string         `json:"data"`
	}{hexutil.EncodeBig(req.ChainID), req.From, req.To, hexutil.EncodeBig(req.Value), hexutil.Encode(req.Data)}
	err := w.post(ctx, "/send-transaction", input, &response)
	if response.Error != "" {
		err = errors.Join(err, errors.New(response.Error))
	}
	return response.Hash, err
}

func envAddress(name string) common.Address {
	value := os.Getenv(name)
	if !common.IsHexAddress(value) || common.HexToAddress(value) == (common.Address{}) {
		log.Fatalf("%s must be a nonzero Ethereum address", name)
	}
	return common.HexToAddress(value)
}

func main() {
	send := flag.Bool("send", false, "authorize a live ERC-20 transfer")
	flag.Parse()
	endpoint, err := url.Parse(os.Getenv("WALLET_URL"))
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil {
		log.Fatal("WALLET_URL must be HTTPS without embedded credentials")
	}
	if os.Getenv("WALLET_TOKEN") == "" {
		log.Fatal("WALLET_TOKEN required")
	}
	wallet := &remoteWallet{
		address: envAddress("WALLET_ADDRESS"),
		url:     strings.TrimRight(endpoint.String(), "/"),
		token:   os.Getenv("WALLET_TOKEN"),
		http: &http.Client{
			Timeout: 30 * time.Second,
			// Do not replay a wallet authorization POST at a redirect target.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	client, err := clob.NewSignerClient(
		clob.Config{ChainID: clob.PolygonChainID, Signer: wallet, RPCURL: os.Getenv("RPC_URL")},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("EOA:", client.Address())
	if !*send {
		return
	}
	amount, ok := new(big.Int).SetString(os.Getenv("AMOUNT"), 10)
	if !ok || amount.Sign() <= 0 {
		log.Fatal("AMOUNT must be positive base units")
	}
	// This executable owns the root context; SDK and backend calls share it.
	ctx, stop := signal.NotifyContext(context.TODO(), os.Interrupt)
	defer stop()
	receipt, err := client.TransferERC20(
		ctx,
		clob.ERC20TransferRequest{
			TokenAddress:     envAddress("TOKEN_ADDRESS"),
			RecipientAddress: envAddress("RECIPIENT_ADDRESS"),
			Amount:           amount,
		},
	)
	if err != nil {
		var sendErr *clob.WalletTransactionError
		if errors.As(err, &sendErr) {
			log.Printf("reconcile before retrying: %+v", sendErr.Submission)
		}
		log.Fatal(err)
	}
	fmt.Println("Mined:", receipt.Hash.Hex())
}
