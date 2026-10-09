package data

func (t *TraderLeaderboardEntry) UnmarshalJSON(raw []byte) error {
	type wire TraderLeaderboardEntry
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.UserName, &value.ProfileImage, &value.XUsername)
	*t = TraderLeaderboardEntry(value)
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

func (b *BuilderStanding) UnmarshalJSON(raw []byte) error {
	type wire BuilderStanding
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.ProfileImage)
	*b = BuilderStanding(value)
	return nil
}

func (b *BuilderVolumePoint) UnmarshalJSON(raw []byte) error {
	type wire BuilderVolumePoint
	var value wire
	if err := decodeWire(raw, &value); err != nil {
		return err
	}
	clearEmptyText(&value.ProfileImage)
	*b = BuilderVolumePoint(value)
	return nil
}
