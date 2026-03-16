package data

// Position represents an open or closed market position.
type Position struct {
	AssetAddress string  `json:"assetAddress"`
	ConditionID  string  `json:"conditionId"`
	Size         float64 `json:"size"`
	AvgPrice     float64 `json:"avgPrice"`
	CurPrice     float64 `json:"curPrice"`
	PNL          float64 `json:"pnl"`
	Unrealized   float64 `json:"unrealized"`
	Realized     float64 `json:"realized"`
	MarketTitle  string  `json:"marketTitle"`
	Outcome      string  `json:"outcome,omitzero"`
}

// PositionValue represents the total value of a user's positions.
type PositionValue struct {
	Value float64 `json:"value"`
}

// DataTrade represents a historical trade record from the Data API.
type DataTrade struct {
	ID              string  `json:"id"`
	MarketID        string  `json:"marketId"`
	AssetAddress    string  `json:"assetAddress"`
	User            string  `json:"user"`
	Side            string  `json:"side"` // BUY, SELL
	Size            float64 `json:"size"`
	Price           float64 `json:"price"`
	TransactionHash string  `json:"transactionHash"`
	Timestamp       int64   `json:"timestamp"`
}

// Activity represents an on-chain action (merge, split, redemption).
type Activity struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"` // MERGE, SPLIT, REDEMPTION
	User            string  `json:"user"`
	MarketID        string  `json:"marketId"`
	AssetAddress    string  `json:"assetAddress"`
	Amount          float64 `json:"amount"`
	TransactionHash string  `json:"transactionHash"`
	Timestamp       int64   `json:"timestamp"`
}

// LeaderboardEntry represents a single user's rank on a leaderboard.
type LeaderboardEntry struct {
	Rank         int     `json:"rank"`
	ProxyWallet  string  `json:"proxyWallet"`
	Username     string  `json:"userName"`
	Volume       float64 `json:"vol"`
	PNL          float64 `json:"pnl,omitzero"`
	ProfileImage string  `json:"profileImage,omitzero"`
	XUsername    string  `json:"xUsername,omitzero"`
	Verified     bool    `json:"verifiedBadge,omitzero"`
}

// LeaderboardCategory defines valid leaderboard categories.
type LeaderboardCategory string

const (
	CategoryOverall  LeaderboardCategory = "OVERALL"
	CategoryPolitics LeaderboardCategory = "POLITICS"
	CategorySports   LeaderboardCategory = "SPORTS"
	CategoryCrypto   LeaderboardCategory = "CRYPTO"
	CategoryCulture  LeaderboardCategory = "CULTURE"
)

// LeaderboardTimePeriod defines valid leaderboard time windows.
type LeaderboardTimePeriod string

const (
	PeriodDay   LeaderboardTimePeriod = "DAY"
	PeriodWeek  LeaderboardTimePeriod = "WEEK"
	PeriodMonth LeaderboardTimePeriod = "MONTH"
	PeriodAll   LeaderboardTimePeriod = "ALL"
)

// LeaderboardSort defines leaderboard sorting criteria.
type LeaderboardSort string

const (
	SortPNL LeaderboardSort = "PNL"
	SortVol LeaderboardSort = "VOL"
)

// LeaderboardParams filters leaderboard requests.
type LeaderboardParams struct {
	Category   LeaderboardCategory
	TimePeriod LeaderboardTimePeriod
	SortBy     LeaderboardSort
	Limit      int
	Offset     int
	User       string
	UserName   string
}

// TradeParams filters Data API trade requests.
type TradeParams struct {
	User   string
	Market string
	Limit  int
	Offset int
}
