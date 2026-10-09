package data

func (e *ComboPositionMarketEvent) UnmarshalJSON(raw []byte) error {
	type wire ComboPositionMarketEvent
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.EventSlug, &value.EventTitle, &value.EventImage)
	*e = ComboPositionMarketEvent(value)
	return nil
}

func (m *ComboPositionMarket) UnmarshalJSON(raw []byte) error {
	type wire ComboPositionMarket
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.Slug, &value.Title, &value.Question, &value.GroupItemTitle,
		&value.SportsMarketType, &value.Outcome, &value.ImageURL, &value.IconURL,
		&value.Category, &value.Subcategory)
	*m = ComboPositionMarket(value)
	return nil
}

func (l *ComboPositionLeg) UnmarshalJSON(raw []byte) error {
	type wire ComboPositionLeg
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.LegOutcomeLabel)
	*l = ComboPositionLeg(value)
	return nil
}

func (p *ComboPosition) UnmarshalJSON(raw []byte) error {
	type wire ComboPosition
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	id, err := normalizeComboConditionID(value.ConditionID)
	if err != nil {
		return err
	}
	value.ConditionID = id
	*p = ComboPosition(value)
	return nil
}
