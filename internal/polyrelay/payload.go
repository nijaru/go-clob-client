package polyrelay

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// SubmitRequest is the /submit request body. The three wallet schemes populate
// different subsets, discriminated by Type; omitempty drops the unused fields.
// All numeric fields are string-encoded on the wire (the relayer expects
// decimal strings, not JSON numbers), so the DTO uses string fields and the
// builders perform the *big.Int -> decimal and []byte -> 0x-hex conversions.
type SubmitRequest struct {
	Type            string               `json:"type"`
	From            string               `json:"from"`
	To              string               `json:"to"`
	ProxyWallet     string               `json:"proxyWallet,omitempty"`
	Data            string               `json:"data,omitempty"`
	Value           string               `json:"value,omitempty"`
	Nonce           string               `json:"nonce,omitempty"`
	Signature       string               `json:"signature,omitempty"`
	Metadata        string               `json:"metadata,omitempty"`
	SignatureParams *signatureParams     `json:"signatureParams,omitempty"`
	DepositWallet   *depositWalletParams `json:"depositWalletParams,omitempty"`
}

// signatureParams carries the scheme-specific signed parameters the relayer
// echoes back into on-chain submission. PROXY uses gas fields; SAFE uses the
// SafeTx gas/operation fields.
type signatureParams struct {
	GasLimit       string `json:"gasLimit,omitempty"`
	GasPrice       string `json:"gasPrice,omitempty"`
	Relay          string `json:"relay,omitempty"`
	RelayHub       string `json:"relayHub,omitempty"`
	RelayerFee     string `json:"relayerFee,omitempty"`
	BaseGas        string `json:"baseGas,omitempty"`
	GasToken       string `json:"gasToken,omitempty"`
	Operation      string `json:"operation,omitempty"`
	RefundReceiver string `json:"refundReceiver,omitempty"`
	SafeTxnGas     string `json:"safeTxnGas,omitempty"`
}

// depositWalletParams carries the deposit-wallet batch the relayer submits.
type depositWalletParams struct {
	DepositWallet string        `json:"depositWallet"`
	Deadline      string        `json:"deadline"`
	Calls         []depositCall `json:"calls"`
}

type depositCall struct {
	Target string `json:"target"`
	Value  string `json:"value"`
	Data   string `json:"data"`
}

// hexData converts raw bytes to a 0x-prefixed hex string ("0x" for empty).
func hexData(b []byte) string {
	return "0x" + common.Bytes2Hex(b)
}

func addrHex(a common.Address) string { return a.Hex() }

func bigStr(v *big.Int) (string, error) {
	if v == nil {
		return "", fmt.Errorf("%w: big.Int field", ErrNilValue)
	}
	return v.String(), nil
}

// --- PROXY ---

// ProxySubmitInput holds the typed inputs for a PROXY /submit body.
type ProxySubmitInput struct {
	Signer, ProxyFactory, Wallet common.Address
	Data                         []byte // EncodeProxyCall output
	Nonce, GasLimit              *big.Int
	Signature                    []byte // Sign(TransactionTypeProxy, ...) output
	Relay, RelayHub              common.Address
	Metadata                     string
}

// BuildProxySubmit assembles the PROXY /submit body from typed inputs.
func BuildProxySubmit(in ProxySubmitInput) (*SubmitRequest, error) {
	nonce, err := bigStr(in.Nonce)
	if err != nil {
		return nil, err
	}
	gasLimit, err := bigStr(in.GasLimit)
	if err != nil {
		return nil, err
	}
	sig, err := HexSignature(in.Signature)
	if err != nil {
		return nil, err
	}
	return &SubmitRequest{
		Type:        string(TransactionTypeProxy),
		From:        addrHex(in.Signer),
		To:          addrHex(in.ProxyFactory),
		ProxyWallet: addrHex(in.Wallet),
		Data:        hexData(in.Data),
		Nonce:       nonce,
		Signature:   sig,
		Metadata:    in.Metadata,
		SignatureParams: &signatureParams{
			GasLimit:   gasLimit,
			GasPrice:   "0",
			Relay:      addrHex(in.Relay),
			RelayHub:   addrHex(in.RelayHub),
			RelayerFee: "0",
		},
	}, nil
}

// --- SAFE ---

// SafeSubmitInput holds the typed inputs for a SAFE /submit body.
type SafeSubmitInput struct {
	Signer, Wallet, Target common.Address
	Data                   []byte // EncodeSafeMultisendCall output (or single-call data)
	Value                  *big.Int
	Operation              uint8
	Nonce                  *big.Int
	Signature              []byte
	Metadata               string
}

// BuildSafeSubmit assembles the SAFE /submit body. Value is included only when
// non-zero, matching the reference SDK.
func BuildSafeSubmit(in SafeSubmitInput) (*SubmitRequest, error) {
	if in.Value == nil {
		return nil, fmt.Errorf("%w: safe value", ErrNilValue)
	}
	nonce, err := bigStr(in.Nonce)
	if err != nil {
		return nil, err
	}
	sig, err := HexSignature(in.Signature)
	if err != nil {
		return nil, err
	}
	req := &SubmitRequest{
		Type:        string(TransactionTypeSafe),
		From:        addrHex(in.Signer),
		To:          addrHex(in.Target),
		ProxyWallet: addrHex(in.Wallet),
		Data:        hexData(in.Data),
		Nonce:       nonce,
		Signature:   sig,
		Metadata:    in.Metadata,
		SignatureParams: &signatureParams{
			BaseGas:        "0",
			GasPrice:       "0",
			GasToken:       addrHex(common.Address{}),
			Operation:      fmt.Sprintf("%d", in.Operation),
			RefundReceiver: addrHex(common.Address{}),
			SafeTxnGas:     "0",
		},
	}
	if in.Value.Sign() > 0 {
		req.Value = in.Value.String()
	}
	return req, nil
}

// --- WALLET (deposit) ---

// DepositSubmitInput holds the typed inputs for a WALLET /submit body.
type DepositSubmitInput struct {
	Signer, Factory, Wallet common.Address
	Calls                   []TransactionCall
	Nonce, Deadline         *big.Int
	Signature               []byte
	Metadata                string
}

// BuildDepositSubmit assembles the WALLET (deposit) /submit body. Calls are
// carried raw in depositWalletParams (no multiSend encoding).
func BuildDepositSubmit(in DepositSubmitInput) (*SubmitRequest, error) {
	if len(in.Calls) == 0 {
		return nil, ErrEmptyBatch
	}
	nonce, err := bigStr(in.Nonce)
	if err != nil {
		return nil, err
	}
	deadline, err := bigStr(in.Deadline)
	if err != nil {
		return nil, err
	}
	sig, err := HexSignature(in.Signature)
	if err != nil {
		return nil, err
	}
	calls := make([]depositCall, len(in.Calls))
	for i, c := range in.Calls {
		val, err := bigStr(c.Value)
		if err != nil {
			return nil, fmt.Errorf("call %d: %w", i, err)
		}
		calls[i] = depositCall{Target: addrHex(c.To), Value: val, Data: hexData(c.Data)}
	}
	return &SubmitRequest{
		Type:      string(TransactionTypeWallet),
		From:      addrHex(in.Signer),
		To:        addrHex(in.Factory),
		Nonce:     nonce,
		Signature: sig,
		Metadata:  in.Metadata,
		DepositWallet: &depositWalletParams{
			DepositWallet: addrHex(in.Wallet),
			Deadline:      deadline,
			Calls:         calls,
		},
	}, nil
}

// --- WALLET-CREATE (deploy, relayed unsigned) ---

// WalletCreateInput holds the typed inputs for a WALLET-CREATE /submit body.
type WalletCreateInput struct {
	Signer, Factory common.Address
	Metadata        string
}

// BuildWalletCreate assembles the WALLET-CREATE deploy body (no signature).
func BuildWalletCreate(in WalletCreateInput) *SubmitRequest {
	return &SubmitRequest{
		Type:     string(TransactionTypeWalletCreate),
		From:     addrHex(in.Signer),
		To:       addrHex(in.Factory),
		Metadata: in.Metadata,
	}
}
