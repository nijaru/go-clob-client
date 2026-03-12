package clob

const (
	timeEndpoint = "/time"

	createAPIKeyEndpoint = "/auth/api-key"
	getAPIKeysEndpoint   = "/auth/api-keys"
	deleteAPIKeyEndpoint = "/auth/api-key"
	deriveAPIKeyEndpoint = "/auth/derive-api-key"
	closedOnlyEndpoint   = "/auth/ban-status/closed-only"

	samplingSimplifiedMarketsEndpoint = "/sampling-simplified-markets"
	samplingMarketsEndpoint           = "/sampling-markets"
	simplifiedMarketsEndpoint         = "/simplified-markets"
	marketsEndpoint                   = "/markets"
	marketEndpoint                    = "/markets/"
	orderBookEndpoint                 = "/book"
	orderBooksEndpoint                = "/books"
	midpointEndpoint                  = "/midpoint"
	midpointsEndpoint                 = "/midpoints"
	priceEndpoint                     = "/price"
	pricesEndpoint                    = "/prices"
	spreadEndpoint                    = "/spread"
	spreadsEndpoint                   = "/spreads"
	lastTradePriceEndpoint            = "/last-trade-price"
	lastTradesPricesEndpoint          = "/last-trades-prices"
	tickSizeEndpoint                  = "/tick-size"
	negRiskEndpoint                   = "/neg-risk"
	feeRateEndpoint                   = "/fee-rate"

	postOrderEndpoint    = "/order"
	postOrdersEndpoint   = "/orders"
	cancelOrderEndpoint  = "/order"
	cancelOrdersEndpoint = "/orders"
	orderEndpoint        = "/data/order/"
	cancelAllEndpoint    = "/cancel-all"
	openOrdersEndpoint   = "/data/orders"
	tradesEndpoint       = "/data/trades"
)
