package clob

import (
	"fmt"
	"math/big"
)

func validateCTFPartition(partition []*big.Int, amount *big.Int) error {
	if err := validateUint256(amount, "position amount"); err != nil {
		return err
	}
	if len(partition) < 2 {
		return fmt.Errorf("ctf: partition requires at least two index sets")
	}
	union := new(big.Int)
	for _, index := range partition {
		if err := validateUint256(index, "partition index set"); err != nil {
			return err
		}
		if index.Sign() == 0 || new(big.Int).And(union, index).Sign() != 0 {
			return fmt.Errorf("ctf: partition must contain nonzero disjoint index sets")
		}
		union.Or(union, index)
	}
	return nil
}
