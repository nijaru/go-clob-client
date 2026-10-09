package polyrelay

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// CorrectDepositNonce re-signs a deposit batch with the on-chain nonce reported
// by a stale execute-params response. It preserves calls, deadline and metadata;
// a fresh params fetch can return the same stale nonce indefinitely.
// It returns nil,nil when the error is not a deposit nonce mismatch.
func CorrectDepositNonce(
	cfg GaslessConfig,
	key *ecdsa.PrivateKey,
	payload *SubmitRequest,
	submitErr error,
) (*SubmitRequest, error) {
	if payload.Type != string(TransactionTypeWallet) {
		return nil, nil
	}
	var apiErr *polyhttp.APIError
	if !errors.As(submitErr, &apiErr) || apiErr.StatusCode != 400 {
		return nil, nil
	}
	match := nonceMismatchRE.FindStringSubmatch(apiErr.Message)
	if match == nil {
		return nil, nil
	}
	submitted, ok := new(big.Int).SetString(match[1], 10)
	if !ok {
		return nil, fmt.Errorf("polyrelay: invalid submitted nonce")
	}
	nonce, ok := new(big.Int).SetString(match[2], 10)
	if !ok {
		return nil, fmt.Errorf("polyrelay: invalid corrected nonce")
	}
	// Only stale nonces are retryable. A future/equal nonce must not be
	// silently replaced, nor may an error for another batch change this one.
	if submitted.String() != payload.Nonce || submitted.Cmp(nonce) >= 0 {
		return nil, nil
	}
	if _, err := pad32(nonce); err != nil {
		return nil, err
	}
	batch := payload.DepositWallet
	if batch == nil {
		return nil, fmt.Errorf("polyrelay: deposit request missing batch")
	}
	deadline, ok := new(big.Int).SetString(batch.Deadline, 10)
	if !ok {
		return nil, fmt.Errorf("polyrelay: invalid deposit deadline")
	}
	calls := make([]TransactionCall, len(batch.Calls))
	for i, call := range batch.Calls {
		value, ok := new(big.Int).SetString(call.Value, 10)
		if !ok {
			return nil, fmt.Errorf("polyrelay: invalid call value")
		}
		data, err := hexutil.Decode(call.Data)
		if err != nil {
			return nil, err
		}
		calls[i] = TransactionCall{To: common.HexToAddress(call.Target), Value: value, Data: data}
	}
	sig, err := Sign(
		TransactionTypeWallet,
		key,
		RelayRequest{
			Wallet:   cfg.Wallet,
			ChainID:  cfg.ChainID,
			Nonce:    nonce,
			Deadline: deadline,
			Calls:    calls,
		},
	)
	if err != nil {
		return nil, err
	}
	if cfg.SessionSigner {
		sig, err = WrapSessionSignature(cfg.Signer, sig)
		if err != nil {
			return nil, err
		}
	}
	corrected := *payload
	corrected.Nonce = nonce.String()
	corrected.Signature = hexData(sig)
	return &corrected, nil
}
