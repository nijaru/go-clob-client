package data

func clearEmptyText(fields ...**string) {
	for _, field := range fields {
		if *field != nil && **field == "" {
			*field = nil
		}
	}
}

func clearAbsentTime(field **Timestamp) {
	if *field != nil && ((*field).IsZero() || (*field).Unix() == 0) {
		*field = nil
	}
}

func (p *Position) UnmarshalJSON(raw []byte) error {
	type wire Position
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.Title, &value.Slug, &value.Icon, &value.EventSlug,
		&value.Outcome, &value.OppositeOutcome, &value.Name, &value.ProfileImage,
		&value.OppositeAssetID, &value.EndDate)
	if value.EndDate != nil && *value.EndDate == "1970-01-01" {
		value.EndDate = nil
	}
	if value.EventID != nil && (*value.EventID == "" || *value.EventID == "0") {
		value.EventID = nil
	}
	if value.OutcomeIndex != nil && *value.OutcomeIndex == 999 {
		value.OutcomeIndex = nil
	}
	clearAbsentTime(&value.LastEventAt)
	clearAbsentTime(&value.FirstEntryAt)
	*p = Position(value)
	return nil
}

func (u *UserStats) UnmarshalJSON(raw []byte) error {
	type wire UserStats
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearAbsentTime(&value.JoinDate)
	*u = UserStats(value)
	return nil
}

func (h *Holder) UnmarshalJSON(raw []byte) error {
	type wire Holder
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	if value.OutcomeIndex != nil && *value.OutcomeIndex == 999 {
		value.OutcomeIndex = nil
	}
	clearEmptyText(
		&value.Name,
		&value.Pseudonym,
		&value.Bio,
		&value.ProfileImage,
		&value.ProfileImageOptimized,
	)
	*h = Holder(value)
	return nil
}

func (t *TraderLeaderboardStanding) UnmarshalJSON(raw []byte) error {
	type wire TraderLeaderboardStanding
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	if value.PnLRank != nil && *value.PnLRank == 0 {
		value.PnLRank = nil
	}
	if value.VolumeRank != nil && *value.VolumeRank == 0 {
		value.VolumeRank = nil
	}
	clearEmptyText(&value.UserName, &value.ProfileImage, &value.XUsername)
	*t = TraderLeaderboardStanding(value)
	return nil
}

func (t *Trade) UnmarshalJSON(raw []byte) error {
	type wire Trade
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	if value.OutcomeIndex != nil && *value.OutcomeIndex == 999 {
		value.OutcomeIndex = nil
	}
	clearEmptyText(
		&value.Title,
		&value.Slug,
		&value.Icon,
		&value.EventSlug,
		&value.Outcome,
		&value.Name,
		&value.Pseudonym,
		&value.Bio,
		&value.ProfileImage,
		&value.ProfileImageOptimized,
	)
	*t = Trade(value)
	return nil
}

func (o *OpenInterest) UnmarshalJSON(raw []byte) error {
	type wire OpenInterest
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	if value.ConditionID != nil && *value.ConditionID == "GLOBAL" {
		value.ConditionID = nil
	}
	*o = OpenInterest(value)
	return nil
}
