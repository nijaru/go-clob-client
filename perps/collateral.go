package perps

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// TransactionCall is an unsigned EVM call. External senders own its execution;
// CollateralWallet provides explicit approval, submission and receipt operations
// using the existing CLOB wallet engine.
type TransactionCall struct {
	To    common.Address
	Data  []byte
	Value *big.Int
}

func (c *OwnerClient) PrepareDeposit(amount *big.Int) (TransactionCall, error) {
	if c.token == (common.Address{}) || c.deposit == (common.Address{}) {
		return TransactionCall{}, fmt.Errorf(
			"perps: collateral token and deposit contract required",
		)
	}
	if amount == nil || amount.Sign() <= 0 || amount.BitLen() > 256 {
		return TransactionCall{}, fmt.Errorf("perps: amount must be positive uint256")
	}
	data := append([]byte(nil), crypto.Keccak256([]byte("deposit(address,uint256,address)"))[:4]...)
	data = append(data, common.LeftPadBytes(c.token.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(c.signer.Address().Bytes(), 32)...)
	return TransactionCall{To: c.deposit, Data: data, Value: new(big.Int)}, nil
}

// TransactionSender is the wallet boundary for EOA or relayed contract calls.
// Implementations own gas, signing, submission and receipt tracking.
type TransactionSender interface {
	SendCall(context.Context, TransactionCall) (common.Hash, error)
}

// Deposit sends one prepared call through the caller's wallet. It never grants
// spending approval or retries an uncertain transaction submission.
func (c *OwnerClient) Deposit(
	ctx context.Context,
	amount *big.Int,
	sender TransactionSender,
) (common.Hash, error) {
	if sender == nil {
		return common.Hash{}, fmt.Errorf("perps: transaction sender required")
	}
	call, err := c.PrepareDeposit(amount)
	if err != nil {
		return common.Hash{}, err
	}
	return sender.SendCall(ctx, call)
}

func (c *OwnerClient) Withdraw(ctx context.Context, amount *big.Int) (int64, error) {
	if c.token == (common.Address{}) || c.deposit == (common.Address{}) {
		return 0, fmt.Errorf("perps: collateral token and deposit contract required")
	}
	if amount == nil || amount.Sign() <= 0 || amount.BitLen() > 256 {
		return 0, fmt.Errorf("perps: amount must be positive uint256")
	}
	salt, err := randomPerpsSalt()
	if err != nil {
		return 0, err
	}
	ts := time.Now().Unix()
	account, token, to := c.signer.Address().Hex(), c.token.Hex(), c.wallet.Hex()
	data := perpsTypedData(
		c.chainID,
		"Withdraw",
		[]apitypes.Type{
			{Name: "account", Type: "address"},
			{Name: "token", Type: "address"},
			{Name: "amount", Type: "uint256"},
			{Name: "fee", Type: "uint256"},
			{Name: "to", Type: "address"},
			{Name: "salt", Type: "uint64"},
			{Name: "ts", Type: "uint64"},
		},
		apitypes.TypedDataMessage{
			"account": account,
			"token":   token,
			"amount":  amount.String(),
			"fee":     "0",
			"to":      to,
			"salt":    strconv.FormatUint(salt, 10),
			"ts":      strconv.FormatInt(ts, 10),
		},
		c.deposit.Hex(),
	)
	sig, err := c.signer.SignTypedData(ctx, data)
	if err != nil {
		return 0, err
	}
	body := map[string]any{
		"op": map[string]any{
			"type": "withdraw",
			"args": map[string]any{
				"account": account,
				"token":   token,
				"amount":  amount.String(),
				"to":      to,
			},
		},
		"salt": salt,
		"sig":  sig,
		"ts":   ts,
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		ID     *int64 `json:"withdraw_id"`
	}
	if err := c.submit(ctx, http.MethodPost, "/v1/account/withdraw", body, &out); err != nil {
		return 0, err
	}
	if err := commandRejection("withdraw", out.Status, out.Error); err != nil {
		return 0, err
	}
	if out.ID == nil {
		return 0, fmt.Errorf("perps: withdrawal ID missing")
	}
	return *out.ID, nil
}

type TransferCollateralRequest struct{ Recipient, Amount, Label string }

func (c *OwnerClient) TransferCollateral(
	ctx context.Context,
	request TransferCollateralRequest,
) (int64, error) {
	if c.token == (common.Address{}) {
		return 0, fmt.Errorf("perps: collateral token required")
	}
	if !common.IsHexAddress(request.Recipient) ||
		common.HexToAddress(request.Recipient) == (common.Address{}) ||
		common.HexToAddress(request.Recipient) == c.signer.Address() {
		return 0, fmt.Errorf("perps: invalid transfer recipient")
	}
	if len(request.Label) > 64 || !utf8.ValidString(request.Label) {
		return 0, fmt.Errorf("perps: transfer label exceeds 64 UTF-8 bytes")
	}
	if _, err := parseFixedDecimal(request.Amount, true); err != nil {
		return 0, err
	}
	account, token := c.signer.Address().Hex(), c.token.Hex()
	body, err := c.signedOperation(
		ctx,
		"internalTransfer",
		[]any{account, token, request.Amount, request.Recipient},
		map[string]any{
			"account": account,
			"token":   token,
			"amount":  request.Amount,
			"to":      request.Recipient,
		},
	)
	if err != nil {
		return 0, err
	}
	if request.Label != "" {
		body["label"] = request.Label
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		ID     *int64 `json:"transfer_id"`
	}
	if err := c.submit(ctx, http.MethodPost, "/v1/account/internal-transfer", body, &out); err != nil {
		return 0, err
	}
	if err := commandRejection("internalTransfer", out.Status, out.Error); err != nil {
		return 0, err
	}
	if out.ID == nil {
		return 0, fmt.Errorf("perps: transfer ID missing")
	}
	return *out.ID, nil
}
