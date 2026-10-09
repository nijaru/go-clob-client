package clob

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var walletABI = mustWalletABI(`[
{"type":"function","name":"authorizeSessionSigner","inputs":[{"type":"address"},{"type":"uint256"}],"outputs":[]},
{"type":"function","name":"revokeSessionSigner","inputs":[{"type":"address"}],"outputs":[]},
{"type":"function","name":"split","inputs":[{"type":"bytes31"},{"type":"uint256"}],"outputs":[]},
{"type":"function","name":"merge","inputs":[{"type":"bytes31"},{"type":"uint256"}],"outputs":[]},
{"type":"function","name":"redeem","inputs":[{"type":"bytes31"},{"type":"uint256"},{"type":"uint256"}],"outputs":[]},
{"type":"function","name":"prepareCondition","inputs":[{"type":"uint256[]"}],"outputs":[]},
{"type":"function","name":"balanceOf","inputs":[{"type":"address"},{"type":"uint256"}],"outputs":[{"type":"uint256"}]},
{"type":"function","name":"balanceOfBatch","inputs":[{"type":"address[]"},{"type":"uint256[]"}],"outputs":[{"type":"uint256[]"}]}
]`)

func mustWalletABI(source string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(source))
	if err != nil {
		panic("clob: wallet ABI: " + err.Error())
	}
	return parsed
}

func packSessionKeyCall(address common.Address, expiry *int64) ([]byte, error) {
	if expiry == nil {
		return walletABI.Pack("revokeSessionSigner", address)
	}
	if *expiry < 0 {
		return nil, fmt.Errorf("%w: expiry", ErrInvalidSessionKey)
	}
	return walletABI.Pack("authorizeSessionSigner", address, big.NewInt(*expiry))
}

func packV2PositionCall(
	method string,
	condition V2ConditionID,
	amount *big.Int,
	outcome uint8,
) ([]byte, error) {
	if err := validateUint256(amount, "position amount"); err != nil {
		return nil, err
	}
	if method == "redeem" {
		if outcome > 1 {
			return nil, fmt.Errorf("%w: outcome must be YES/NO", ErrInvalidPositionOperation)
		}
		return walletABI.Pack(
			method,
			[31]byte(condition),
			new(big.Int).SetUint64(uint64(outcome)),
			amount,
		)
	}
	return walletABI.Pack(method, [31]byte(condition), amount)
}
