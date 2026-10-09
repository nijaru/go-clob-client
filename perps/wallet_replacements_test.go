package perps

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/signing"
)

// This provider signs real transactions independently of the Wallet verifier.
// The original nonce association is provider-owned, never inferred from an
// opaque hash by the SDK. Increased fees produce a distinct completion hash.
type replacementProvider struct {
	key       *ecdsa.PrivateKey
	mu        sync.Mutex
	originals map[common.Hash]*types.Transaction
	order     []common.Hash
	mode      string
	cancel    context.CancelFunc
}

func (p *replacementProvider) Address() common.Address {
	return crypto.PubkeyToAddress(p.key.PublicKey)
}

func (p *replacementProvider) SignTypedData(context.Context, apitypes.TypedData) ([]byte, error) {
	return nil, errors.New("unexpected signing prompt")
}

func (p *replacementProvider) SendTransaction(
	_ context.Context,
	r signing.TransactionRequest,
) (common.Hash, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID: r.ChainID, Nonce: uint64(len(p.order)), To: &r.To, Value: r.Value,
		Data: r.Data, Gas: 100000, GasFeeCap: big.NewInt(10), GasTipCap: big.NewInt(1),
	}), types.LatestSignerForChainID(new(big.Int).Set(r.ChainID)), p.key)
	if err != nil {
		return common.Hash{}, err
	}
	p.originals[tx.Hash()] = tx
	p.order = append(p.order, tx.Hash())
	// Prove that send requests cannot mutate the pinned completion intent.
	r.ChainID.SetInt64(1)
	r.Value.SetInt64(99)
	r.Data[0] ^= 255
	if p.mode == "uncertain final" && len(p.order) == 2 {
		return tx.Hash(), errors.New("lost send acknowledgement")
	}
	return tx.Hash(), nil
}

func (p *replacementProvider) WaitTransaction(
	ctx context.Context,
	r signing.TransactionRequest,
	original common.Hash,
) (*types.Transaction, *types.Receipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := p.originals[original]
	if o == nil || r.From != p.Address() || r.ChainID.Cmp(o.ChainId()) != 0 || r.To != *o.To() ||
		r.Value.Cmp(o.Value()) != 0 ||
		!bytes.Equal(r.Data, o.Data()) {
		return nil, nil, errors.New("completion intent changed or original hash lost")
	}
	chain, to, value, data, key := o.ChainId(), *o.To(), o.Value(), o.Data(), p.key
	// Most defects occur in the final call, preserving the confirmed prefix.
	if o.Nonce() == 1 {
		switch p.mode {
		case "recipient":
			to = common.HexToAddress("0x999")
		case "value":
			value = big.NewInt(1)
		case "data":
			data = []byte{9}
		case "cancel transaction":
			to, value, data = p.Address(), new(big.Int), nil
		case "chain":
			chain = big.NewInt(1)
		case "sender":
			key, _ = crypto.GenerateKey()
		}
	}
	tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID: chain, Nonce: o.Nonce(), To: &to, Value: value, Data: data,
		Gas: 100000, GasFeeCap: big.NewInt(20), GasTipCap: big.NewInt(2),
	}), types.LatestSignerForChainID(chain), key)
	if err != nil {
		return nil, nil, err
	}
	receipt := &types.Receipt{
		TxHash:      tx.Hash(),
		BlockHash:   common.HexToHash("0xbeef"),
		BlockNumber: big.NewInt(42),
		Status:      1,
	}
	if p.mode == "reverted prefix" || p.mode == "reverted final" && o.Nonce() == 1 {
		receipt.Status = 0
	}
	if o.Nonce() == 1 {
		switch p.mode {
		case "receipt hash":
			receipt.TxHash = original
		case "unmined":
			receipt.BlockNumber = nil
		case "status":
			receipt.Status = 2
		case "context":
			p.cancel()
		}
	}
	// Both callback requests are independent copies, including validation's
	// expected request: provider mutation cannot redefine acceptable intent.
	r.ChainID.SetInt64(999)
	r.Value.SetInt64(99)
	r.Data[0] ^= 255
	return tx, receipt, ctx.Err()
}

func TestCollateralWalletReplacementCompletion(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"success", "recipient", "value", "data", "chain", "sender", "cancel transaction", "receipt hash", "unmined", "status", "reverted prefix", "reverted final", "context", "uncertain final"} {
		t.Run(mode, func(t *testing.T) {
			f, rpc := newCollateralRPC(t)
			key, err := crypto.HexToECDSA(fixturePrivateKey)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			provider := &replacementProvider{
				key:       key,
				originals: map[common.Hash]*types.Transaction{},
				mode:      mode,
				cancel:    cancel,
			}
			boundary, err := signing.NewWallet(provider)
			if err != nil {
				t.Fatal(err)
			}
			cfg := collateralConfig(t, rpc, clob.SignatureTypeEOA)
			cfg.Owner.Signer = boundary
			cfg.Transactions.PrivateKey = ""
			cfg.Transactions.Signer = boundary // exercise NewWallet reuse in the real entry point
			wallet := mustCollateralWallet(t, cfg)
			handle, sendErr := wallet.ApproveAndDeposit(ctx, big.NewInt(100000000), "")
			if handle == nil {
				t.Fatalf("lost partial handle: %v", sendErr)
			}
			if mode == "reverted prefix" {
				if !errors.Is(sendErr, ErrCollateralTransactionReverted) ||
					len(provider.order) != 1 ||
					handle.Submissions[0].ConfirmedReceipt == nil {
					t.Fatalf("lost reverted prefix: %+v %v", handle, sendErr)
				}
			} else if mode == "uncertain final" {
				if sendErr == nil || !handle.Submissions[1].BroadcastUncertain {
					t.Fatal("lost send uncertainty")
				}
			} else if sendErr != nil {
				t.Fatal(sendErr)
			}
			receipts, waitErr := handle.Wait(ctx)
			wantCount := 1
			switch mode {
			case "success", "uncertain final":
				wantCount = 2
				if waitErr != nil {
					t.Fatal(waitErr)
				}
			case "reverted prefix", "reverted final":
				if mode == "reverted final" {
					wantCount = 2
				}
				if !errors.Is(waitErr, ErrCollateralTransactionReverted) {
					t.Fatalf("revert error lost: %v", waitErr)
				}
			case "context":
				wantCount = 2
				if !errors.Is(waitErr, context.Canceled) {
					t.Fatalf("cancellation lost: %v", waitErr)
				}
			default:
				if !errors.Is(waitErr, signing.ErrInvalidTransactionCompletion) {
					t.Fatalf("changed intent/receipt accepted: %v", waitErr)
				}
			}
			if len(receipts) != wantCount {
				t.Fatalf("prefix/receipt lost: %d %v", len(receipts), waitErr)
			}
			for i, receipt := range receipts {
				if handle.Submissions[i].TransactionHash != provider.order[i].Hex() ||
					receipt.TxHash == provider.order[i] {
					t.Fatal("original submission changed or replacement receipt hash lost")
				}
			}
			if mode == "reverted prefix" {
				f.chain.Store(1)
				prefix, err := handle.Wait(t.Context())
				if len(prefix) != 1 || prefix[0].Status != 0 ||
					!errors.Is(err, ErrCollateralTransactionReverted) {
					t.Fatalf("failed chain recheck lost known revert: %d %v", len(prefix), err)
				}
			}
			if f.sent.Load() != 0 || f.receipts.Load() != 0 {
				t.Fatal("provider completion performed RPC broadcast/polling")
			}
			if mode == "success" {
				var wg sync.WaitGroup
				for range 3 {
					wg.Go(func() {
						r, err := handle.Wait(t.Context())
						if err != nil || len(r) != 2 {
							t.Errorf("concurrent wait: %d %v", len(r), err)
						}
					})
				}
				wg.Wait()
				if len(provider.order) != 2 {
					t.Fatal("wait resent calls")
				}
				cancel()
				prefix, err := handle.Wait(ctx)
				if !errors.Is(err, context.Canceled) || len(prefix) != 1 ||
					prefix[0].TxHash != receipts[0].TxHash {
					t.Fatalf("pre-cancelled wait lost known prefix: %d %v", len(prefix), err)
				}
			}
		})
	}
}
