package gamma

// OutcomeAsset pairs an outcome's wire index and label with its quoted price
// and identifiers. Price's empty value and nil IDs mean the service did not
// supply that value; no token ID is substituted for a native position ID.
type OutcomeAsset struct {
	Index      int
	Label      string
	Price      Decimal
	TokenID    *string
	PositionID *string
}

// OutcomeDetails returns all outcomes in wire order, including legacy markets
// with more than two outcomes. Short or absent price/identifier arrays leave the
// corresponding value absent, matching discovery of markets not yet tradable.
// The returned identifiers do not alias the market's arrays.
func (m Market) OutcomeDetails() []OutcomeAsset {
	if m.Outcomes == nil {
		return nil
	}
	assets := make([]OutcomeAsset, len(m.Outcomes))
	for i, label := range m.Outcomes {
		assets[i] = OutcomeAsset{Index: i, Label: label}
		if i < len(m.OutcomePrices) {
			assets[i].Price = Decimal(m.OutcomePrices[i])
		}
		if i < len(m.CLOBTokenIDs) {
			assets[i].TokenID = new(m.CLOBTokenIDs[i])
		}
		if i < len(m.PositionIDs) {
			assets[i].PositionID = new(m.PositionIDs[i])
		}
	}
	return assets
}
