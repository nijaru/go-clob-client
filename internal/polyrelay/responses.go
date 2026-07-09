package polyrelay

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// ExecuteParams is the relayer's response to a nonce (or relay-payload) fetch.
// The proxy path uses Address as the relay address to sign against; Safe and
// deposit paths use only the Nonce.
type ExecuteParams struct {
	Address common.Address
	Nonce   *big.Int
}

// ExecuteResponse is the immediate response to POST /submit. TransactionHash is
// empty ("") when the relayer has not yet produced one; callers fall back to the
// polled hash.
type ExecuteResponse struct {
	State           RelayerTransactionState
	TransactionHash string
	TransactionID   string
}

// GaslessTransaction is a polled transaction status from
// GET /v1/account/transactions/{id}. Note the snake_case wire keys
// (transaction_hash, transaction_id, error_msg), which differ from the
// camelCase /submit response.
type GaslessTransaction struct {
	State           RelayerTransactionState
	TransactionHash string
	TransactionID   string
	ErrorMsg        string
}

// DeployedResponse reports whether a wallet is deployed on-chain.
type DeployedResponse struct {
	Deployed bool
}

// TransactionOutcome is the terminal result of polling a gasless transaction.
type TransactionOutcome struct {
	TransactionHash string
	TransactionID   string
}

// --- private wire shapes (raw JSON; conversion to typed Go happens in transport.go) ---

type executeParamsWire struct {
	Address string `json:"address"`
	Nonce   string `json:"nonce"`
}

type executeResponseWire struct {
	State           string `json:"state"`
	TransactionHash string `json:"transactionHash"`
	TransactionID   string `json:"transactionID"`
}

type gaslessTransactionWire struct {
	State           string `json:"state"`
	TransactionHash string `json:"transaction_hash"`
	TransactionID   string `json:"transaction_id"`
	ErrorMsg        string `json:"error_msg"`
}

type deployedWire struct {
	Deployed bool `json:"deployed"`
}

// parseExecuteParams converts the raw nonce string into a *big.Int, validating
// it is a non-empty decimal (the relayer contract requires a numeric nonce).
func parseExecuteParams(w executeParamsWire) (ExecuteParams, error) {
	if w.Nonce == "" {
		return ExecuteParams{}, errInvalidNonce
	}
	n, ok := new(big.Int).SetString(w.Nonce, 10)
	if !ok {
		return ExecuteParams{}, errInvalidNonce
	}
	return ExecuteParams{Address: common.HexToAddress(w.Address), Nonce: n}, nil
}
