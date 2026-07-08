package polyrelay

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// TransactionCall is a single EVM call bundled into a gasless relay request.
// Data holds the raw calldata (no 0x prefix). Value is denominated in wei.
type TransactionCall struct {
	To    common.Address
	Data  []byte
	Value *big.Int
}

// RelayerTransactionType identifies the wallet family a relayed transaction
// targets. Wire values mirror the relayer API (note the hyphenated forms).
type RelayerTransactionType string

const (
	// TransactionTypeSafe targets a Polymarket Gnosis Safe wallet.
	TransactionTypeSafe RelayerTransactionType = "SAFE"
	// TransactionTypeProxy targets a legacy Polymarket proxy wallet.
	TransactionTypeProxy RelayerTransactionType = "PROXY"
	// TransactionTypeSafeCreate deploys a new Safe wallet via relay.
	TransactionTypeSafeCreate RelayerTransactionType = "SAFE-CREATE"
	// TransactionTypeWallet targets a Solady deposit wallet.
	TransactionTypeWallet RelayerTransactionType = "WALLET"
	// TransactionTypeWalletCreate deploys a new deposit wallet via relay.
	TransactionTypeWalletCreate RelayerTransactionType = "WALLET-CREATE"
)

// RelayerTransactionState is the lifecycle state of a relayed transaction.
type RelayerTransactionState string

const (
	TransactionStateNew       RelayerTransactionState = "STATE_NEW"
	TransactionStateExecuted  RelayerTransactionState = "STATE_EXECUTED"
	TransactionStateMined     RelayerTransactionState = "STATE_MINED"
	TransactionStateConfirmed RelayerTransactionState = "STATE_CONFIRMED"
	TransactionStateInvalid   RelayerTransactionState = "STATE_INVALID"
	TransactionStateFailed    RelayerTransactionState = "STATE_FAILED"
)

// IsTerminal reports whether state is a terminal outcome worth stopping a poll loop.
func (s RelayerTransactionState) IsTerminal() bool {
	switch s {
	case TransactionStateConfirmed, TransactionStateFailed, TransactionStateInvalid:
		return true
	}
	return false
}
