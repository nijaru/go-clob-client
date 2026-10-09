package data

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

type ApprovalStandard string

const (
	ApprovalERC20   ApprovalStandard = "ERC20"
	ApprovalERC1155 ApprovalStandard = "ERC1155"
)

// ApprovalAmount is either "max" or a canonical unsigned uint256 base-unit
// amount. Indexed approvals can lag on-chain state; Approved must also be true.
type ApprovalAmount string

const ApprovalMax ApprovalAmount = "max"

func (a *ApprovalAmount) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value != "max" {
		if value == "" || len(value) > 78 || (len(value) > 1 && value[0] == '0') ||
			strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return fmt.Errorf("data: invalid approval amount %q", value)
		}
		amount, ok := new(big.Int).SetString(value, 10)
		if !ok || amount.BitLen() > 256 {
			return fmt.Errorf("data: approval amount exceeds uint256")
		}
	}
	*a = ApprovalAmount(value)
	return nil
}

// ApprovalContract is a catalog row. Unknown standards keep Raw and their
// addresses, but are never considered approved by the SDK. Known malformed
// rows fail decoding rather than being reclassified as unknown.
type ApprovalContract struct {
	Token    string           `json:"token"`
	Spender  string           `json:"spender"`
	Standard ApprovalStandard `json:"standard"`
	Amount   *ApprovalAmount  `json:"amount"`
	Approved bool             `json:"approved"`
	Raw      json.RawMessage  `json:"-"`
}

func (a *ApprovalContract) UnmarshalJSON(raw []byte) error {
	var header struct {
		Token    *string          `json:"token"`
		Spender  *string          `json:"spender"`
		Standard ApprovalStandard `json:"standard"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return err
	}
	if header.Token == nil || header.Spender == nil {
		return fmt.Errorf("data: approval row is missing addresses")
	}
	value := ApprovalContract{
		Token:    *header.Token,
		Spender:  *header.Spender,
		Standard: header.Standard,
	}
	if header.Standard != ApprovalERC20 && header.Standard != ApprovalERC1155 {
		value.Raw = append(json.RawMessage(nil), raw...)
		*a = value
		return nil
	}
	if !common.IsHexAddress(value.Token) || !common.IsHexAddress(value.Spender) {
		return fmt.Errorf("data: invalid approval row address")
	}
	var known struct {
		Amount   *ApprovalAmount `json:"amount"`
		Approved *bool           `json:"approved"`
	}
	if err := json.Unmarshal(raw, &known); err != nil {
		return err
	}
	if known.Approved == nil || (header.Standard == ApprovalERC20 && known.Amount == nil) {
		return fmt.Errorf("data: incomplete %s approval row", header.Standard)
	}
	value.Amount, value.Approved = known.Amount, *known.Approved
	*a = value
	return nil
}

// ApprovalsSnapshot is an indexed, possibly stale allowance snapshot for one
// wallet and chain. It is not evidence that an approval transaction confirmed.
type ApprovalsSnapshot struct {
	Address   common.Address     `json:"address"`
	ChainID   int64              `json:"chain_id"`
	Contracts []ApprovalContract `json:"contracts"`
}

func (a *ApprovalsSnapshot) UnmarshalJSON(raw []byte) error {
	var wire struct {
		Address   *common.Address     `json:"address"`
		ChainID   *int64              `json:"chain_id"`
		Contracts *[]ApprovalContract `json:"contracts"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	if wire.Address == nil || wire.ChainID == nil || *wire.ChainID < 1 || wire.Contracts == nil {
		return fmt.Errorf("data: incomplete approval snapshot")
	}
	*a = ApprovalsSnapshot{
		Address:   *wire.Address,
		ChainID:   *wire.ChainID,
		Contracts: *wire.Contracts,
	}
	return nil
}
