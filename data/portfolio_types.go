package data

// PortfolioValue is returned by the Data API v2.
type PortfolioValue struct {
	Wallet string        `json:"proxy_wallet" required:"true"`
	Value  DecimalString `json:"value"        required:"true"`
}

// UserPnLPoint is returned by the Data API v2.
type UserPnLPoint struct {
	Timestamp         Timestamp      `json:"timestamp"           required:"true"`
	SourceBlock       int64          `json:"source_block"        required:"true"`
	TradeCount        int64          `json:"trade_count"         required:"true"`
	RealizedMarketPnL DecimalString  `json:"realized_market_pnl" required:"true"`
	RealizedLPPnL     DecimalString  `json:"realized_lp_pnl"     required:"true"`
	RealizedComboPnL  DecimalString  `json:"realized_combo_pnl"  required:"true"`
	RealizedPnL       DecimalString  `json:"realized_pnl"        required:"true"`
	Volume            DecimalString  `json:"volume"              required:"true"`
	VolumeUSDC        DecimalString  `json:"volume_usdc"         required:"true"`
	UnrealizedPnL     *DecimalString `json:"unrealized_pnl"`
	FeesRefunded      *DecimalString `json:"fees_refunded"`
	MakerRebate       *DecimalString `json:"maker_rebate"`
	TakerRebate       *DecimalString `json:"taker_rebate"`
	RewardIncome      *DecimalString `json:"reward_income"`
	YieldIncome       *DecimalString `json:"yield_income"`
	ReferralIncome    *DecimalString `json:"referral_income"`
	SponsoredIncome   *DecimalString `json:"sponsored_income"`
	Deposits          *DecimalString `json:"deposits"`
	Withdrawals       *DecimalString `json:"withdrawals"`
	CashflowNet       *DecimalString `json:"cashflow_net"`
	WalletIncome      *DecimalString `json:"wallet_income"`
	PositionPnL       *DecimalString `json:"position_pnl"`
	SettledPnL        *DecimalString `json:"settled_pnl"`
	EconomicPnL       *DecimalString `json:"economic_pnl"`
	TradePnL          *DecimalString `json:"trade_pnl"`
	Fees              *DecimalString `json:"fees"`
	FeesPaid          *DecimalString `json:"fees_paid"`
}

// UserStats is returned by the Data API v2.
type UserStats struct {
	Wallet            string        `json:"proxy_wallet" required:"true"`
	TradedMarketCount int64         `json:"trades"       required:"true"`
	BiggestWin        DecimalString `json:"biggest_win"  required:"true"`
	Views             int64         `json:"views"        required:"true"`
	JoinDate          *Timestamp    `json:"join_date"`
	AllTimePnL        *UserPnLPoint `json:"all_time_pnl"`
}

// UserPnLSeries is returned by the Data API v2.
type UserPnLSeries struct {
	Wallet         string          `json:"proxy_wallet"    required:"true"`
	Interval       UserPnLInterval `json:"interval"        required:"true"`
	Fidelity       UserPnLFidelity `json:"fidelity"        required:"true"`
	SourceFidelity UserPnLFidelity `json:"source_fidelity" required:"true"`
	Points         []UserPnLPoint  `json:"points"          required:"true"`
}

// UserVolume is returned by the Data API v2.
type UserVolume struct {
	Volume     DecimalString `json:"volume"      required:"true"`
	VolumeUSDC DecimalString `json:"volume_usdc" required:"true"`
	TradeCount int64         `json:"trade_count" required:"true"`
}
