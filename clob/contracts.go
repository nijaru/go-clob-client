package clob

import "fmt"

const zeroAddress = "0x0000000000000000000000000000000000000000"

type ContractConfig struct {
	Exchange        string
	NegRiskExchange string
	NegRiskAdapter  string
	Collateral      string
	Conditional     string
}

var contractConfigs = map[int64]ContractConfig{
	137: {
		Exchange:        "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E",
		NegRiskExchange: "0xC5d563A36AE78145C45a50134d48A1215220f80a",
		NegRiskAdapter:  "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		Collateral:      "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
		Conditional:     "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045",
	},
	80002: {
		Exchange:        "0xdFE02Eb6733538f8Ea35D585af8DE5958AD99E40",
		NegRiskExchange: "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		NegRiskAdapter:  "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		Collateral:      "0x9c4e1703476e875070ee25b56a58b008cfb8fa78",
		Conditional:     "0x69308FB512518e39F9b16112fA8d994F4e2Bf8bB",
	},
}

func getContractConfig(chainID int64) (ContractConfig, error) {
	config, ok := contractConfigs[chainID]
	if !ok {
		return ContractConfig{}, fmt.Errorf("unsupported chain id %d", chainID)
	}
	return config, nil
}
