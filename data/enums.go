package data

type ActivityType string

const (
	ActivityTypeTrade          ActivityType = "TRADE"
	ActivityTypeSplit          ActivityType = "SPLIT"
	ActivityTypeMerge          ActivityType = "MERGE"
	ActivityTypeRedeem         ActivityType = "REDEEM"
	ActivityTypeReward         ActivityType = "REWARD"
	ActivityTypeConversion     ActivityType = "CONVERSION"
	ActivityTypeMigration      ActivityType = "MIGRATION"
	ActivityTypeDeposit        ActivityType = "DEPOSIT"
	ActivityTypeWithdrawal     ActivityType = "WITHDRAWAL"
	ActivityTypeYield          ActivityType = "YIELD"
	ActivityTypeMakerRebate    ActivityType = "MAKER_REBATE"
	ActivityTypeTakerRebate    ActivityType = "TAKER_REBATE"
	ActivityTypeReferralReward ActivityType = "REFERRAL_REWARD"
	ActivityTypeTip            ActivityType = "TIP"
)

type ComboActivityType string

const (
	ComboActivityTypeSplit    ComboActivityType = "SPLIT"
	ComboActivityTypeMerge    ComboActivityType = "MERGE"
	ComboActivityTypeConvert  ComboActivityType = "CONVERT"
	ComboActivityTypeCompress ComboActivityType = "COMPRESS"
	ComboActivityTypeWrap     ComboActivityType = "WRAP"
	ComboActivityTypeUnwrap   ComboActivityType = "UNWRAP"
	ComboActivityTypeRedeem   ComboActivityType = "REDEEM"
)

type TipSide string

const (
	TipSideIn  TipSide = "IN"
	TipSideOut TipSide = "OUT"
)

type PositionStatus string

const (
	PositionStatusOpen           PositionStatus = "OPEN"
	PositionStatusRedeemable     PositionStatus = "REDEEMABLE"
	PositionStatusRedeemableLost PositionStatus = "REDEEMABLE_LOST"
	PositionStatusMergeable      PositionStatus = "MERGEABLE"
	PositionStatusClosed         PositionStatus = "CLOSED"
)

type ComboPositionStatus string

const (
	ComboPositionStatusOpen            ComboPositionStatus = "OPEN"
	ComboPositionStatusRedeemable      ComboPositionStatus = "REDEEMABLE"
	ComboPositionStatusPartial         ComboPositionStatus = "PARTIAL"
	ComboPositionStatusResolvedPartial ComboPositionStatus = "RESOLVED_PARTIAL"
	ComboPositionStatusResolvedWin     ComboPositionStatus = "RESOLVED_WIN"
	ComboPositionStatusResolvedLoss    ComboPositionStatus = "RESOLVED_LOSS"
)

type BiggestWinnerKind string

const (
	BiggestWinnerKindMarket BiggestWinnerKind = "market"
	BiggestWinnerKindCombo  BiggestWinnerKind = "combo"
)

type UserPnLInterval string

const (
	UserPnLIntervalMax         UserPnLInterval = "max"
	UserPnLIntervalAll         UserPnLInterval = "all"
	UserPnLIntervalOneMonth    UserPnLInterval = "1m"
	UserPnLIntervalOneWeek     UserPnLInterval = "1w"
	UserPnLIntervalOneDay      UserPnLInterval = "1d"
	UserPnLIntervalTwelveHours UserPnLInterval = "12h"
	UserPnLIntervalSixHours    UserPnLInterval = "6h"
)

type UserPnLFidelity string

const (
	UserPnLFidelityOneDay        UserPnLFidelity = "1d"
	UserPnLFidelityEighteenHours UserPnLFidelity = "18h"
	UserPnLFidelityTwelveHours   UserPnLFidelity = "12h"
	UserPnLFidelityThreeHours    UserPnLFidelity = "3h"
	UserPnLFidelityOneHour       UserPnLFidelity = "1h"
)

type ResolutionStatus string

const (
	ResolutionStatusInitialized ResolutionStatus = "initialized"
	ResolutionStatusPosed       ResolutionStatus = "posed"
	ResolutionStatusProposed    ResolutionStatus = "proposed"
	ResolutionStatusChallenged  ResolutionStatus = "challenged"
	ResolutionStatusReproposed  ResolutionStatus = "reproposed"
	ResolutionStatusDisputed    ResolutionStatus = "disputed"
	ResolutionStatusActive      ResolutionStatus = "active"
	ResolutionStatusArbitration ResolutionStatus = "arbitration"
	ResolutionStatusResolved    ResolutionStatus = "resolved"
)

type ResolutionSettlementTimeBasis string

const (
	ResolutionSettlementTimeBasisManagedProposalExpiration ResolutionSettlementTimeBasis = "managed_proposal_expiration"
	ResolutionSettlementTimeBasisProposalExpiration        ResolutionSettlementTimeBasis = "proposal_expiration"
	ResolutionSettlementTimeBasisLiveness                  ResolutionSettlementTimeBasis = "liveness"
	ResolutionSettlementTimeBasisDvmRoundEstimate          ResolutionSettlementTimeBasis = "dvm_round_estimate"
)

type ResolutionMarketType string

const (
	ResolutionMarketTypeBinary             ResolutionMarketType = "BINARY"
	ResolutionMarketTypeIncrementalNegrisk ResolutionMarketType = "INCREMENTAL_NEGRISK"
	ResolutionMarketTypeAtomicNegrisk      ResolutionMarketType = "ATOMIC_NEGRISK"
)

type ResolutionSource string

const (
	ResolutionSourceReported ResolutionSource = "reported"
	ResolutionSourceDerived  ResolutionSource = "derived"
)

type ResolutionReporter string

const (
	ResolutionReporterUmaOptimisticOracle ResolutionReporter = "UMA_OO"
	ResolutionReporterChainlink           ResolutionReporter = "CHAINLINK"
	ResolutionReporterEoa                 ResolutionReporter = "EOA"
)

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type SortDirection string

const (
	SortAscending  SortDirection = "ASC"
	SortDescending SortDirection = "DESC"
)

type FilterType string

const (
	FilterCash   FilterType = "CASH"
	FilterTokens FilterType = "TOKENS"
)

type PositionSortBy string

const (
	PositionSortCurrentValue  PositionSortBy = "CURRENT_VALUE"
	PositionSortPrice         PositionSortBy = "PRICE"
	PositionSortTokens        PositionSortBy = "TOKENS"
	PositionSortUnrealizedPnL PositionSortBy = "UNREALIZED_PNL"
	PositionSortRealizedPnL   PositionSortBy = "REALIZED_PNL"
	PositionSortTotalPnL      PositionSortBy = "TOTAL_PNL"
	PositionSortTimestamp     PositionSortBy = "TIMESTAMP"
)

type ComboPositionSortBy string

const (
	ComboSortFirstEntry   ComboPositionSortBy = "FIRST_ENTRY"
	ComboSortEntryCost    ComboPositionSortBy = "ENTRY_COST"
	ComboSortCurrentValue ComboPositionSortBy = "CURRENT_VALUE"
	ComboSortUpdated      ComboPositionSortBy = "UPDATED"
)

type LeaderboardWindow string

const (
	WindowDay   LeaderboardWindow = "day"
	WindowWeek  LeaderboardWindow = "week"
	WindowMonth LeaderboardWindow = "month"
	WindowAll   LeaderboardWindow = "all"
)

type LeaderboardSort string

const (
	LeaderboardSortPnL    LeaderboardSort = "PNL"
	LeaderboardSortVolume LeaderboardSort = "VOLUME"
)

type BuilderVolumeInterval string

const (
	BuilderVolumeDay   BuilderVolumeInterval = "day"
	BuilderVolumeWeek  BuilderVolumeInterval = "week"
	BuilderVolumeMonth BuilderVolumeInterval = "month"
	BuilderVolumeAll   BuilderVolumeInterval = "all"
)

type PriceHistoryInterval string

const (
	PriceHistoryMax      PriceHistoryInterval = "max"
	PriceHistoryAll      PriceHistoryInterval = "all"
	PriceHistoryMonth    PriceHistoryInterval = "1m"
	PriceHistoryWeek     PriceHistoryInterval = "1w"
	PriceHistoryDay      PriceHistoryInterval = "1d"
	PriceHistorySixHours PriceHistoryInterval = "6h"
	PriceHistoryHour     PriceHistoryInterval = "1h"
)
