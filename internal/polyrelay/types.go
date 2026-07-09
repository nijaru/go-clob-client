package polyrelay

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// TransactionCall is a single EVM call bundled into a gasless relay request.
// Data holds raw calldata; Value is denominated in wei.
type TransactionCall struct {
	To    common.Address
	Data  []byte
	Value *big.Int
}

// RelayerTransactionType identifies the wallet family a relayed transaction
// targets. Wire values mirror the relayer API (note the hyphenated forms).
type RelayerTransactionType string

const (
	// TransactionTypeProxy targets a legacy Polymarket proxy wallet.
	TransactionTypeProxy RelayerTransactionType = "PROXY"
	// TransactionTypeSafe targets a Polymarket Gnosis Safe wallet.
	TransactionTypeSafe RelayerTransactionType = "SAFE"
	// TransactionTypeWallet targets a Solady deposit wallet.
	TransactionTypeWallet RelayerTransactionType = "WALLET"
	// TransactionTypeSafeCreate deploys a new Safe via relay.
	TransactionTypeSafeCreate RelayerTransactionType = "SAFE-CREATE"
	// TransactionTypeWalletCreate deploys a new deposit wallet via relay.
	TransactionTypeWalletCreate RelayerTransactionType = "WALLET-CREATE"
)

// RelayerTransactionState is the lifecycle state of a relayed transaction.
type RelayerTransactionState string

const (
	StateNew       RelayerTransactionState = "STATE_NEW"
	StateExecuted  RelayerTransactionState = "STATE_EXECUTED"
	StateMined     RelayerTransactionState = "STATE_MINED"
	StateConfirmed RelayerTransactionState = "STATE_CONFIRMED"
	StateInvalid   RelayerTransactionState = "STATE_INVALID"
	StateFailed    RelayerTransactionState = "STATE_FAILED"
)

// IsTerminal reports whether a poll loop should stop at this state.
func (s RelayerTransactionState) IsTerminal() bool {
	switch s {
	case StateConfirmed, StateFailed, StateInvalid:
		return true
	}
	return false
}

// RelayRequest carries everything the signers need. The schemes read different
// subsets — callers populate only the fields relevant to their transaction type:
//
//   - PROXY:  Signer, To, Data, Nonce, and the Gas*/Relay* fields.
//   - SAFE:   Wallet (verifying contract), To, Data, Value, Operation, Nonce, ChainID.
//   - WALLET: Wallet, Calls, Nonce, Deadline, ChainID.
//
// This union is deliberate: PROXY and SAFE sign a single prepared transaction
// (calls are pre-encoded into Data via multiSend by the transport), while
// WALLET (deposit) signs the raw batch natively. Splitting per scheme would
// scatter dispatch; one documented struct keeps the entry point uniform.
type RelayRequest struct {
	// Shared
	Signer  common.Address // EOA authorizing the relay (proxy "from")
	Wallet  common.Address // verifying contract: Safe address / deposit wallet
	ChainID *big.Int       // EIP-712 domain chain ID (SAFE, WALLET)
	Nonce   *big.Int       // relayer execute nonce

	// Single-transaction schemes (PROXY, SAFE)
	To        common.Address
	Data      []byte
	Value     *big.Int
	Operation uint8

	// PROXY relay/gas parameters
	RelayHub common.Address
	Relay    common.Address
	GasFee   *big.Int
	GasPrice *big.Int
	GasLimit *big.Int

	// WALLET (deposit) batch + deadline
	Calls    []TransactionCall
	Deadline *big.Int
}

// Sentinels for invalid request inputs. Use errors.Is to distinguish.
var (
	ErrNilKey        = errors.New("polyrelay: nil private key")
	ErrEmptyBatch    = errors.New("polyrelay: empty call batch")
	ErrUnknownType   = errors.New("polyrelay: unknown relayer transaction type")
	ErrNilValue      = errors.New("polyrelay: nil value in uint256 field")
	ErrNegativeValue = errors.New("polyrelay: negative value in uint256 field")
	ErrOverflow      = errors.New("polyrelay: value exceeds uint256")

	// Transport / orchestration errors.
	ErrEmptyCalls         = errors.New("polyrelay: gasless submission requires at least one call")
	ErrMetadataTooLong    = errors.New("polyrelay: metadata exceeds maximum length")
	ErrTransactionFailed  = errors.New("polyrelay: transaction reached a terminal failure state")
	ErrTransactionTimeout = errors.New("polyrelay: timed out waiting for transaction confirmation")
	ErrNoTransactionHash  = errors.New("polyrelay: transaction confirmed without a transaction hash")
)
