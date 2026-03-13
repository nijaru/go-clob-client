package clob

// GetCollateralAddress returns the configured collateral token address for the client's chain.
func (c *Client) GetCollateralAddress() (string, error) {
	config, err := getContractConfig(c.chainID)
	if err != nil {
		return "", err
	}
	return config.Collateral, nil
}

// GetConditionalAddress returns the configured conditional-tokens contract address for the client's chain.
func (c *Client) GetConditionalAddress() (string, error) {
	config, err := getContractConfig(c.chainID)
	if err != nil {
		return "", err
	}
	return config.Conditional, nil
}

// GetExchangeAddress returns the configured exchange address for the client's chain.
func (c *Client) GetExchangeAddress(negRisk bool) (string, error) {
	config, err := getContractConfig(c.chainID)
	if err != nil {
		return "", err
	}
	if negRisk {
		return config.NegRiskExchange, nil
	}
	return config.Exchange, nil
}
