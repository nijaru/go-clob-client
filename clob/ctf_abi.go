package clob

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

const ctfABIJSON = `[
  {
    "inputs": [
      {"name":"collateralToken","type":"address"},
      {"name":"parentCollectionId","type":"bytes32"},
      {"name":"conditionId","type":"bytes32"},
      {"name":"partition","type":"uint256[]"},
      {"name":"amount","type":"uint256"}
    ],
    "name":"splitPosition",
    "outputs":[],
    "stateMutability":"nonpayable",
    "type":"function"
  },
  {
    "inputs": [
      {"name":"collateralToken","type":"address"},
      {"name":"parentCollectionId","type":"bytes32"},
      {"name":"conditionId","type":"bytes32"},
      {"name":"partition","type":"uint256[]"},
      {"name":"amount","type":"uint256"}
    ],
    "name":"mergePositions",
    "outputs":[],
    "stateMutability":"nonpayable",
    "type":"function"
  },
  {
    "inputs": [
      {"name":"collateralToken","type":"address"},
      {"name":"parentCollectionId","type":"bytes32"},
      {"name":"conditionId","type":"bytes32"},
      {"name":"indexSets","type":"uint256[]"}
    ],
    "name":"redeemPositions",
    "outputs":[],
    "stateMutability":"nonpayable",
    "type":"function"
  },
  {
    "inputs": [
      {"name":"oracle","type":"address"},
      {"name":"questionId","type":"bytes32"},
      {"name":"outcomeSlotCount","type":"uint256"}
    ],
    "name":"getConditionId",
    "outputs":[{"name":"","type":"bytes32"}],
    "stateMutability":"pure",
    "type":"function"
  },
  {
    "inputs": [
      {"name":"parentCollectionId","type":"bytes32"},
      {"name":"conditionId","type":"bytes32"},
      {"name":"indexSet","type":"uint256"}
    ],
    "name":"getCollectionId",
    "outputs":[{"name":"","type":"bytes32"}],
    "stateMutability":"view",
    "type":"function"
  },
  {
    "inputs": [
      {"name":"collateralToken","type":"address"},
      {"name":"collectionId","type":"bytes32"}
    ],
    "name":"getPositionId",
    "outputs":[{"name":"","type":"uint256"}],
    "stateMutability":"pure",
    "type":"function"
  }
]`

const negRiskAdapterABIJSON = `[
  {
    "inputs": [
      {"name":"conditionId","type":"bytes32"},
      {"name":"amounts","type":"uint256[]"}
    ],
    "name":"redeemPositions",
    "outputs":[],
    "stateMutability":"nonpayable",
    "type":"function"
  }
]`

var ctfABI abi.ABI
var negRiskABI abi.ABI

func init() {
	var err error
	ctfABI, err = abi.JSON(strings.NewReader(ctfABIJSON))
	if err != nil {
		panic("clob: parse CTF ABI: " + err.Error())
	}
	negRiskABI, err = abi.JSON(strings.NewReader(negRiskAdapterABIJSON))
	if err != nil {
		panic("clob: parse NegRisk adapter ABI: " + err.Error())
	}
}
