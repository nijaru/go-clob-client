package clob

// SetFeeRate pre-populates the legacy base-fee cache. Fees are basis points:
// 100 bps is 1%. It does not override V2/V3 market fee curves.
func (c *Client) SetFeeRate(tokenID string, fee FeeRateResponse) {
	c.feeRateMu.Lock()
	defer c.feeRateMu.Unlock()
	c.feeRateGeneration++
	c.feeRateCache[tokenID] = fee
}

// SetFeeRateBPS pre-populates the legacy fee cache from a basis-point value.
func (c *Client) SetFeeRateBPS(tokenID string, fee uint32) {
	c.SetFeeRate(tokenID, FeeRateResponse{BaseFee: fee})
}
