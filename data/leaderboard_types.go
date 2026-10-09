package data

// TraderLeaderboardEntry is returned by the Data API v2.
type TraderLeaderboardEntry struct {
	Rank         int64         `json:"rank"          required:"true"`
	Wallet       string        `json:"user_id"       required:"true"`
	PnL          DecimalString `json:"pnl"           required:"true"`
	Volume       DecimalString `json:"volume"        required:"true"`
	UserName     *string       `json:"user_name"`
	ProfileImage *string       `json:"profile_image"`
	XUsername    *string       `json:"x_username"`
	Verified     bool          `json:"verified"      required:"true"`
}

// TraderLeaderboardStanding is returned by the Data API v2.
type TraderLeaderboardStanding struct {
	Wallet       string        `json:"user_id"       required:"true"`
	PnL          DecimalString `json:"pnl"           required:"true"`
	Volume       DecimalString `json:"volume"        required:"true"`
	UserName     *string       `json:"user_name"`
	ProfileImage *string       `json:"profile_image"`
	XUsername    *string       `json:"x_username"`
	Verified     bool          `json:"verified"      required:"true"`
	PnLRank      *int64        `json:"rank_pnl"`
	VolumeRank   *int64        `json:"rank_volume"`
}

// BuilderStanding is returned by the Data API v2.
type BuilderStanding struct {
	Rank         int64         `json:"rank"          required:"true"`
	BuilderName  string        `json:"builder_name"  required:"true"`
	BuilderCode  string        `json:"builder_code"  required:"true"`
	ProfileImage *string       `json:"profile_image"`
	Verified     bool          `json:"verified"      required:"true"`
	Volume       DecimalString `json:"volume"        required:"true"`
	ActiveUsers  int64         `json:"active_users"  required:"true"`
}

// BuilderVolumePoint is returned by the Data API v2.
type BuilderVolumePoint struct {
	Rank         int64         `json:"rank"          required:"true"`
	BuilderName  string        `json:"builder_name"  required:"true"`
	BuilderCode  string        `json:"builder_code"  required:"true"`
	ProfileImage *string       `json:"profile_image"`
	Verified     bool          `json:"verified"      required:"true"`
	Volume       DecimalString `json:"volume"        required:"true"`
	ActiveUsers  int64         `json:"active_users"  required:"true"`
	BucketDate   string        `json:"date"          required:"true"`
}
