package clob

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/nijaru/go-clob-client/signing"
)

// WalletTransactionHandle retains EOA submission attempts or a relayer batch.
// Unknown external hashes remain uncertain attempts, never successful completion.
// Exported fields are read-only snapshots; hashes always identify the original
// submissions. WaitReceipt(s) and Wait expose the final completion hashes.
type WalletTransactionHandle struct {
	TransactionID   string
	TransactionHash string
	Submissions     []WalletTransactionSubmission
	RequestedCalls  int
	relayer         *GaslessTransactionHandle
	signer          *SignerClient
	attempts        []walletTransactionAttempt
	requestedCalls  int
}

// Each attempt owns its immutable original intent and submission. Public fields
// are reporting snapshots; reconciliation never trusts caller-mutated aliases.
type walletTransactionAttempt struct {
	call       TransactionCall
	submission WalletTransactionSubmission
}

func (h *WalletTransactionHandle) Wait(ctx context.Context) (*TransactionOutcome, error) {
	if h == nil || h.signer == nil {
		return nil, fmt.Errorf("wallet: no transaction to wait for")
	}
	if h.relayer != nil {
		return h.relayer.Wait(ctx)
	}
	receipt, err := h.WaitReceipt(ctx)
	if err != nil {
		return nil, err
	}
	return &TransactionOutcome{TransactionHash: receipt.TxHash.Hex()}, nil
}

// WaitReceipt waits for all submitted EOA calls or the atomic relayer batch and
// returns the last collected receipt, even alongside an error. Use WaitReceipts
// to reconcile the entire sequence. Neither method waits for CLOB indexing,
// reorg finality, or ERC-20 return-value checks.
func (h *WalletTransactionHandle) WaitReceipt(ctx context.Context) (*types.Receipt, error) {
	receipts, err := h.WaitReceipts(ctx)
	if len(receipts) == 0 {
		return nil, err
	}
	return receipts[len(receipts)-1], err
}

func (h *WalletTransactionHandle) WaitReceipts(ctx context.Context) ([]*types.Receipt, error) {
	if h == nil || h.signer == nil {
		return nil, fmt.Errorf("wallet: no transaction to wait for")
	}
	if h.relayer != nil {
		outcome, err := h.relayer.Wait(ctx)
		if err != nil {
			return nil, err
		}
		receipt, err := h.signer.waitWalletTransactionReceipt(ctx, outcome.TransactionHash)
		if receipt == nil {
			return nil, err
		}
		return []*types.Receipt{receipt}, err
	}
	if len(h.attempts) == 0 || h.requestedCalls < len(h.attempts) {
		return nil, fmt.Errorf("wallet: no valid EOA submissions to wait for")
	}
	receipts := make([]*types.Receipt, 0, len(h.attempts))
	for _, attempt := range h.attempts {
		submission := attempt.submission
		if submission.TransactionHash == "" {
			return receipts, &WalletTransactionError{
				Submission: submission,
				Err:        ErrWalletTransactionUnknownHash,
			}
		}
		receipt, err := h.signer.waitWalletCallReceipt(
			ctx,
			attempt.call,
			submission.TransactionHash,
		)
		// A failed recheck cannot erase a receipt observed during execution.
		// Keep its error: prior mining is not a new completion/finality claim.
		if receipt == nil && err != nil {
			receipt = submission.ConfirmedReceipt
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		if err != nil {
			return receipts, err
		}
	}
	if len(h.attempts) != h.requestedCalls {
		return receipts, ErrWalletTransactionIncomplete
	}
	return receipts, nil
}

// WaitWalletTransactionReceipt reconciles a known EOA hash without sending.
func (c *SignerClient) WaitWalletTransactionReceipt(
	ctx context.Context,
	outcome TransactionOutcome,
) (*types.Receipt, error) {
	return c.waitWalletTransactionReceipt(ctx, outcome.TransactionHash)
}

func (c *SignerClient) waitWalletTransactionReceipt(
	ctx context.Context,
	hash string,
) (*types.Receipt, error) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(hash, "0x"))
	if err != nil || !strings.HasPrefix(hash, "0x") || len(decoded) != common.HashLength ||
		common.BytesToHash(decoded) == (common.Hash{}) {
		return nil, fmt.Errorf("wallet: receipt requires a valid transaction hash")
	}
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer ec.Close()
	receipt, err := waitForReceipt(ctx, ec, common.BytesToHash(decoded), "wallet")
	if err != nil {
		return nil, err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return receipt, fmt.Errorf("%w: %s reverted", ErrWalletTransactionFailed, hash)
	}
	return receipt, nil
}

// ExecuteWalletTransaction executes calls in order. Smart wallets relay one
// atomic batch. EOAs confirm each preceding call before broadcasting the last;
// an EOA sequence is not atomic. On error a nonnil handle retains any confirmed
// prefix and every send attempt, including external attempts without a hash.
// Never blindly retry the whole sequence; inspect the handle and reconcile first.
func (c *SignerClient) ExecuteWalletTransaction(
	ctx context.Context,
	calls []TransactionCall,
	metadata string,
) (*WalletTransactionHandle, error) {
	if c.signatureType == SignatureTypeEOA {
		return c.ExecuteEOACalls(ctx, calls)
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("%w: no calls", ErrInvalidPositionOperation)
	}
	for _, call := range calls {
		if err := validateUint256(call.Value, "call value"); err != nil {
			return nil, err
		}
	}
	h, err := c.PrepareGaslessTransaction(ctx, calls, metadata)
	if err != nil {
		return nil, err
	}
	return &WalletTransactionHandle{
		TransactionID:   h.TransactionID,
		TransactionHash: h.TransactionHash,
		RequestedCalls:  len(calls),
		relayer:         h,
		signer:          c,
	}, nil
}

// ExecuteEOACalls executes a non-atomic EOA sequence without CLOB credentials.
// Each preceding call must mine successfully before the next is sent. Errors
// retain all attempts, including an external send with an unknown hash. Receipt
// validation via RPC proves mining status and identity, not an opaque sender's
// intent. Optional wallet completion additionally verifies the signed call.
func (c *SignerClient) ExecuteEOACalls(
	ctx context.Context,
	calls []TransactionCall,
) (*WalletTransactionHandle, error) {
	if err := c.requireEOATokenOperation(); err != nil {
		return nil, err
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("%w: no calls", ErrInvalidPositionOperation)
	}
	// Validate and snapshot the entire intent before any wallet prompt.
	prepared := make([]TransactionCall, len(calls))
	for i, call := range calls {
		if err := validateUint256(call.Value, "call value"); err != nil {
			return nil, err
		}
		prepared[i] = TransactionCall{
			To:    call.To,
			Value: new(big.Int).Set(call.Value),
			Data:  append([]byte(nil), call.Data...),
		}
	}
	handle := &WalletTransactionHandle{
		RequestedCalls: len(prepared),
		requestedCalls: len(prepared),
		signer:         c,
	}
	for i, call := range prepared {
		hash, err := c.broadcastWalletCall(ctx, call)
		var sendErr *WalletTransactionError
		var submission WalletTransactionSubmission
		attempted := errors.As(err, &sendErr) || hash != (common.Hash{})
		if sendErr != nil {
			submission = sendErr.Submission
		} else if hash != (common.Hash{}) {
			submission.TransactionHash = hash.Hex()
		}
		if attempted {
			handle.attempts = append(
				handle.attempts,
				walletTransactionAttempt{call: call, submission: submission},
			)
			handle.Submissions = append(handle.Submissions, submission)
		}
		if hash != (common.Hash{}) {
			handle.TransactionHash = hash.Hex()
		}
		if err != nil {
			if len(handle.Submissions) == 0 {
				handle = nil
			}
			return handle, fmt.Errorf("wallet: broadcast call %d of %d: %w", i+1, len(calls), err)
		}
		if i+1 < len(calls) {
			receipt, err := c.waitWalletCallReceipt(ctx, call, hash.Hex())
			handle.attempts[i].submission.ConfirmedReceipt = receipt
			handle.Submissions[i].ConfirmedReceipt = receipt
			if err != nil {
				return handle, fmt.Errorf("wallet: confirm call %d of %d: %w", i+1, len(calls), err)
			}
		}
	}
	return handle, nil
}

// waitWalletCallReceipt uses verified provider completion only when the original
// call intent is available. Hash-only reconciliation retains strict RPC identity.
func (c *SignerClient) waitWalletCallReceipt(
	ctx context.Context,
	call TransactionCall,
	hash string,
) (*types.Receipt, error) {
	if !c.signer.CanWaitTransactions() {
		return c.waitWalletTransactionReceipt(ctx, hash)
	}
	_, receipt, err := c.signer.WaitTransaction(ctx, signing.TransactionRequest{
		ChainID: big.NewInt(c.chainID), To: call.To, Value: call.Value, Data: call.Data,
	}, common.HexToHash(hash))
	if errors.Is(err, signing.ErrTransactionReverted) {
		err = errors.Join(ErrWalletTransactionFailed, err)
	}
	return receipt, err
}

func (c *SignerClient) broadcastWalletCall(
	ctx context.Context,
	call TransactionCall,
) (common.Hash, error) {
	if err := ctx.Err(); err != nil {
		return common.Hash{}, err
	}
	if err := c.requireEOATokenOperation(); err != nil {
		return common.Hash{}, err
	}
	if err := validateUint256(call.Value, "call value"); err != nil {
		return common.Hash{}, err
	}
	if c.signer.CanSendTransactions() {
		hash, err := c.signer.SendTransaction(ctx, signing.TransactionRequest{
			ChainID: big.NewInt(c.chainID), To: call.To, Value: call.Value, Data: call.Data,
		})
		var sendErr *signing.TransactionSendError
		if errors.As(err, &sendErr) {
			return hash, walletSubmissionError(hash, true, err)
		}
		return hash, err
	}
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wallet: dial rpc: %w", err)
	}
	defer ec.Close()
	from := c.signer.Address()
	nonce, err := ec.PendingNonceAt(ctx, from)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wallet: nonce: %w", err)
	}
	tip, err := ec.SuggestGasTipCap(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wallet: gas tip: %w", err)
	}
	head, err := ec.HeaderByNumber(ctx, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wallet: latest header: %w", err)
	}
	if head.BaseFee == nil {
		return common.Hash{}, fmt.Errorf("wallet: chain does not support EIP-1559")
	}
	fee := new(big.Int).Add(tip, new(big.Int).Mul(head.BaseFee, big.NewInt(2)))
	gas, err := ec.EstimateGas(
		ctx,
		ethereum.CallMsg{
			From:      from,
			To:        &call.To,
			Value:     call.Value,
			Data:      call.Data,
			GasFeeCap: fee,
			GasTipCap: tip,
		},
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wallet: estimate gas: %w", err)
	}
	chainID := big.NewInt(c.chainID)
	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: tip,
			GasFeeCap: fee,
			Gas:       gas,
			To:        &call.To,
			Value:     call.Value,
			Data:      call.Data,
		},
	)
	signed, err := c.signer.SignTransaction(ctx, chainID, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("wallet: sign: %w", err)
	}
	if err := ec.SendTransaction(ctx, signed); err != nil {
		return signed.Hash(), walletSubmissionError(
			signed.Hash(),
			true,
			fmt.Errorf("wallet: broadcast: %w", err),
		)
	}
	return signed.Hash(), nil
}

// WalletTransactionSubmission records an EOA attempt. An empty hash means an
// external wallet may have broadcast but supplied no hash; obtain it from the
// wallet before retrying. A known uncertain hash can be reconciled via receipts.
// ConfirmedReceipt retains a valid mined receipt, including a revert.
type WalletTransactionSubmission struct {
	TransactionHash    string
	BroadcastUncertain bool
	ConfirmedReceipt   *types.Receipt
}

// WalletTransactionError retains a submission when sending or confirmation fails.
// It is also returned by direct token/CTF methods, whose receipt result may be nil.
type WalletTransactionError struct {
	Submission WalletTransactionSubmission
	Err        error
}

func (e *WalletTransactionError) Error() string {
	return fmt.Sprintf("wallet: transaction %q: %v", e.Submission.TransactionHash, e.Err)
}

func (e *WalletTransactionError) Unwrap() error { return e.Err }

func walletSubmissionError(hash common.Hash, uncertain bool, err error) *WalletTransactionError {
	submission := WalletTransactionSubmission{BroadcastUncertain: uncertain}
	if hash != (common.Hash{}) {
		submission.TransactionHash = hash.Hex()
	}
	return &WalletTransactionError{Submission: submission, Err: err}
}

var (
	ErrWalletTransactionIncomplete  = errors.New("wallet: not all requested calls were submitted")
	ErrWalletTransactionUnknownHash = errors.New(
		"wallet: external send hash unknown; reconcile with wallet before retrying",
	)
)
