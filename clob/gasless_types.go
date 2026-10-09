package clob

import "github.com/nijaru/go-clob-client/internal/polyrelay"

// TransactionCall is an EVM call; Value is in wei and Data is raw calldata.
type TransactionCall = polyrelay.TransactionCall

// GaslessTransactionHandle represents a submitted relayer transaction. Wait
// takes a caller-owned context; cancelling it does not cancel the transaction.
type GaslessTransactionHandle = polyrelay.Handle

// TransactionOutcome is a confirmed wallet transaction.
type TransactionOutcome = polyrelay.TransactionOutcome

// RelayerTransport provides typed relayer reads and submissions.
type RelayerTransport = polyrelay.Transport

type (
	RelayerTransactionType  = polyrelay.RelayerTransactionType
	RelayerTransactionState = polyrelay.RelayerTransactionState
	RelayerExecuteParams    = polyrelay.ExecuteParams
	RelayerExecuteResponse  = polyrelay.ExecuteResponse
	RelayerSubmitRequest    = polyrelay.SubmitRequest
	RelayerSignatureParams  = polyrelay.SignatureParams
	DepositWalletParams     = polyrelay.DepositWalletParams
	DepositCall             = polyrelay.DepositCall
	GaslessTransaction      = polyrelay.GaslessTransaction
)

const (
	RelayerTransactionProxy        = polyrelay.TransactionTypeProxy
	RelayerTransactionSafe         = polyrelay.TransactionTypeSafe
	RelayerTransactionWallet       = polyrelay.TransactionTypeWallet
	RelayerTransactionSafeCreate   = polyrelay.TransactionTypeSafeCreate
	RelayerTransactionWalletCreate = polyrelay.TransactionTypeWalletCreate
	RelayerStateNew                = polyrelay.StateNew
	RelayerStateExecuted           = polyrelay.StateExecuted
	RelayerStateMined              = polyrelay.StateMined
	RelayerStateConfirmed          = polyrelay.StateConfirmed
	RelayerStateInvalid            = polyrelay.StateInvalid
	RelayerStateFailed             = polyrelay.StateFailed
)

var (
	ErrWalletTransactionFailed  = polyrelay.ErrTransactionFailed
	ErrWalletTransactionTimeout = polyrelay.ErrTransactionTimeout
)
