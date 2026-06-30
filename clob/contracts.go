package clob

import "fmt"

const zeroAddress = "0x0000000000000000000000000000000000000000"

type contractConfig struct {
	Exchange        string
	NegRiskExchange string
	NegRiskAdapter  string
	Collateral      string
	Conditional     string
}

// Polygon (137) and Amoy (80002) contract configurations.
var contractConfigs = map[int64]contractConfig{
	137: {
		Exchange:        "0xE111180000d2663C0091e4f400237545B87B996B",
		NegRiskExchange: "0xe2222d279d744050d28e00520010520000310F59",
		NegRiskAdapter:  "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		Collateral:      "0xC011a7E12a19f7B1f670d46F03B03f3342E82DFB",
		Conditional:     "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045",
	},
	80002: {
		Exchange:        "0xE111180000d2663C0091e4f400237545B87B996B",
		NegRiskExchange: "0xe2222d279d744050d28e00520010520000310F59",
		NegRiskAdapter:  "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296",
		Collateral:      "0xC011a7E12a19f7B1f670d46F03B03f3342E82DFB",
		Conditional:     "0x69308FB512518e39F9b16112fA8d994F4e2Bf8bB",
	},
}

// Wallet factory contract addresses for CREATE2 address derivation.
type walletConfig struct {
	ProxyFactory                string // EIP-1167 minimal proxy factory (Magic/email wallets); empty if unsupported
	SafeFactory                 string // Gnosis Safe factory
	DepositWalletFactory        string // UUPS+Beacon deposit-wallet factory (Poly1271)
	DepositWalletImplementation string // UUPS deposit-wallet implementation
	DepositWalletBeacon         string // Beacon deposit-wallet beacon
}

var walletConfigs = map[int64]walletConfig{
	137: {
		ProxyFactory:                "0xaB45c5A4B0c941a2F231C04C3f49182e1A254052",
		SafeFactory:                 "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b",
		DepositWalletFactory:        "0x00000000000Fb5C9ADea0298D729A0CB3823Cc07",
		DepositWalletImplementation: "0x58CA52ebe0DadfdF531Cde7062e76746de4Db1eB",
		DepositWalletBeacon:         "0x7A18EDfe055488A3128f01F563e5B479D92ffc3a",
	},
	80002: {
		SafeFactory: "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b",
	},
}

// Init code hashes for CREATE2 wallet derivation.
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
