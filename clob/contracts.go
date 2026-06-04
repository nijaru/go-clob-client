package clob

import "fmt"

const zeroAddress = "0x0000000000000000000000000000000000000000"

type contractConfig struct {
	Exchange          string
	ExchangeV2        string
	NegRiskExchange   string
	NegRiskExchangeV2 string
	NegRiskAdapter    string
	Collateral        string
	CollateralV2      string
	Conditional       string
}

// Polygon (137) and Amoy (80002) contract configurations.
// V2 addresses sourced from rs-clob-client-v2/src/lib.rs.
var contractConfigs = map[int64]contractConfig{
	137: {
		Exchange:          "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E",
		ExchangeV2:        "0xE111180000d2663C0091e4f400237545B87B996B",
		NegRiskExchange:   "0xC5d563A36AE78145C45a50134d48A1215220f80a",
		NegRiskExchangeV2: "0xe2222d279d744050d28e00520010520000310F59",
		NegRiskAdapter:    "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		Collateral:        "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
		CollateralV2:      "0xC011a7E12a19f7B1f670d46F03B03f3342E82DFB",
		Conditional:       "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045",
	},
	80002: {
		Exchange:          "0xdFE02Eb6733538f8Ea35D585af8DE5958AD99E40",
		ExchangeV2:        "0xE111180000d2663C0091e4f400237545B87B996B",
		NegRiskExchange:   "0xC5d563A36AE78145C45a50134d48A1215220f80a",
		NegRiskExchangeV2: "0xe2222d279d744050d28e00520010520000310F59",
		NegRiskAdapter:    "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		Collateral:        "0x9c4e1703476e875070ee25b56a58b008cfb8fa78",
		CollateralV2:      "0xC011a7E12a19f7B1f670d46F03B03f3342E82DFB",
		Conditional:       "0x69308FB512518e39F9b16112fA8d994F4e2Bf8bB",
	},
}

// Wallet factory contract addresses for CREATE2 address derivation.
// Source: https://github.com/Polymarket/builder-relayer-client
type walletConfig struct {
	ProxyFactory string // EIP-1167 minimal proxy factory (Magic/email wallets); empty if unsupported
	SafeFactory  string // Gnosis Safe factory
}

var walletConfigs = map[int64]walletConfig{
	137: {
		ProxyFactory: "0xaB45c5A4B0c941a2F231C04C3f49182e1A254052",
		SafeFactory:  "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b",
	},
	80002: {
		// Proxy factory not supported on Amoy testnet
		SafeFactory: "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b",
	},
}

// Init code hashes for CREATE2 wallet derivation.
// These are the keccak256 hashes of the deployed proxy/safe contract creation code.
const (
	proxyInitCodeHash = "0xd21df8dc65880a8606f09fe0ce3df9b8869287ab0b058be05aa9e8af6330a00b"
	safeInitCodeHash  = "0x2bce2127ff07fb632d16c8347c4ebf501f4841168bed00d9e6ef715ddb6fcecf"
)

func getContractConfig(chainID int64) (contractConfig, error) {
	config, ok := contractConfigs[chainID]
	if !ok {
		return contractConfig{}, fmt.Errorf("unsupported chain id %d", chainID)
	}
	return config, nil
}

func getWalletConfig(chainID int64) (walletConfig, error) {
	config, ok := walletConfigs[chainID]
	if !ok {
		return walletConfig{}, fmt.Errorf("unsupported chain id %d", chainID)
	}
	return config, nil
}
