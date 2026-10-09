package clob

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

// WalletAddress is the account whose tokens and positions are operated on.
func (c *SignerClient) WalletAddress() common.Address {
	if c.signatureType == SignatureTypeEOA {
		return c.signer.Address()
	}
	return common.HexToAddress(c.funderAddress)
}

// IsDepositWalletOwner checks both stable deposit-wallet derivations. Unknown
// wallet/signer pairs use the session-signer path, as in the unified SDKs.
func (c *SignerClient) IsDepositWalletOwner() (bool, error) { return c.isDepositWalletOwner() }

func (c *SignerClient) isDepositWalletOwner() (bool, error) {
	if c.signatureType != SignatureTypePoly1271 {
		return false, nil
	}
	wallet := c.WalletAddress()
	if !common.IsHexAddress(c.funderAddress) || wallet == (common.Address{}) {
		return false, fmt.Errorf("wallet: invalid deposit wallet address")
	}
	uups, err := DeriveUUPSDepositWallet(c.signer.Address(), c.chainID)
	if err != nil {
		return false, err
	}
	beacon, err := DeriveBeaconDepositWallet(c.signer.Address(), c.chainID)
	if err != nil {
		return false, err
	}
	return wallet == uups || wallet == beacon, nil
}

// WrapDepositWalletSessionSignature wraps an externally produced signature for
// a Deposit Wallet session key. Trading/auth callers must also select the
// session signer identity when building the inner signed message.
func WrapDepositWalletSessionSignature(signer common.Address, signature []byte) ([]byte, error) {
	return polyrelay.WrapSessionSignature(signer, signature)
}

// DeriveCurrentDepositWallet reads the factory's beacon selector. Only an EVM
// revert means legacy UUPS; transport and malformed-response errors propagate.
func (c *SignerClient) DeriveCurrentDepositWallet(ctx context.Context) (common.Address, error) {
	wc, err := getWalletConfig(c.chainID)
	if err != nil {
		return common.Address{}, err
	}
	if wc.DepositWalletFactory == "" {
		return common.Address{}, fmt.Errorf(
			"wallet: deposit wallet unsupported on chain %d",
			c.chainID,
		)
	}
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return common.Address{}, err
	}
	defer ec.Close()
	factory := common.HexToAddress(wc.DepositWalletFactory)
	data, err := ec.CallContract(
		ctx,
		ethereum.CallMsg{To: &factory, Data: common.FromHex(factoryBeaconSelector)},
		nil,
	)
	if err != nil {
		var rpcErr rpc.Error
		message := strings.ToLower(err.Error())
		var dataErr rpc.DataError
		if errors.As(err, &dataErr) {
			message += " " + strings.ToLower(fmt.Sprint(dataErr.ErrorData()))
		}
		if !errors.As(err, &rpcErr) ||
			!slices.Contains([]int{3, -32000, -32003, -32015, -32603}, rpcErr.ErrorCode()) ||
			(!strings.Contains(message, "revert") && !strings.Contains(message, "invalid opcode")) {
			return common.Address{}, err
		}
		return DeriveUUPSDepositWallet(c.signer.Address(), c.chainID)
	}
	// Legacy factories may return empty data for the missing beacon selector.
	if len(data) == 0 {
		return DeriveUUPSDepositWallet(c.signer.Address(), c.chainID)
	}
	if len(data) != 32 {
		return common.Address{}, fmt.Errorf(
			"wallet: factory beacon returned %d bytes, want 32",
			len(data),
		)
	}
	if common.BytesToAddress(data[12:]) == (common.Address{}) {
		return DeriveUUPSDepositWallet(c.signer.Address(), c.chainID)
	}
	return DeriveBeaconDepositWallet(c.signer.Address(), c.chainID)
}
