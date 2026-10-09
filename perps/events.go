package perps

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Known session payloads have typed decoders while Data retains the exact wire
// JSON for forward-compatible fields. A resync frame is never a data update.
func decodeEvent[T any](e PerpsSessionEvent, channel string) (*T, error) {
	if e.Channel != channel || e.Resync != nil {
		return nil, fmt.Errorf("perps: event is not a %s update", channel)
	}
	var out T
	if err := json.Unmarshal(e.Data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (e PerpsSessionEvent) AsOrder() (*PerpsOrder, error) {
	wire, err := decodeEvent[perpsOrderUpdate](e, "orders")
	if err != nil {
		return nil, err
	}
	out := wire.order()
	return &out, nil
}

func (e PerpsSessionEvent) AsBalance() (*PerpsBalance, error) {
	return decodeEvent[PerpsBalance](e, "balances")
}

func (e PerpsSessionEvent) AsPortfolio() (*PerpsPortfolio, error) {
	return decodeEvent[PerpsPortfolio](e, "portfolio")
}

func (e PerpsSessionEvent) AsBuilderFills() ([]PerpsBuilderEarning, error) {
	out, err := decodeEvent[[]PerpsBuilderEarning](e, "builderFills")
	if err != nil {
		return nil, err
	}
	return *out, nil
}

// PerpsFillUpdate is a live fill. Unlike history records it includes an optional
// client ID and no transaction hash. Fees retain exact exchange/builder totals.
type PerpsFillUpdate struct {
	PerpsAccountFill
	ClientOrderID string
}

func (e PerpsSessionEvent) AsFills() ([]PerpsFillUpdate, error) {
	type fillWire struct {
		TradeID            int64              `json:"tid"`
		OrderID            int                `json:"oid"`
		InstrumentID       int                `json:"iid"`
		Side               PerpsSide          `json:"side"`
		Settlement         bool               `json:"settlement"`
		Price              string             `json:"p"`
		Quantity           string             `json:"qty"`
		Taker              bool               `json:"taker"`
		Fee                string             `json:"fee"`
		BuilderFee         string             `json:"builder_fee"`
		TotalFee           string             `json:"total_fee"`
		Builder            *PerpsBuilderTerms `json:"builder"`
		FeeAsset           string             `json:"fea"`
		PreviousSize       string             `json:"psz"`
		PreviousEntryPrice string             `json:"pep"`
		PnL                string             `json:"pnl"`
		Liquidation        bool               `json:"liq"`
		Timestamp          int64              `json:"ts"`
		ClientID           string             `json:"coid"`
	}
	wire, err := decodeEvent[[]fillWire](e, "fills")
	if err != nil {
		return nil, err
	}
	out := make([]PerpsFillUpdate, len(*wire))
	for i, f := range *wire {
		if f.BuilderFee == "" {
			f.BuilderFee = "0"
		}
		if f.TotalFee == "" {
			value, err := totalFillFee(f.Fee, f.BuilderFee)
			if err != nil {
				return nil, err
			}
			f.TotalFee = value
		}
		out[i] = PerpsFillUpdate{
			PerpsAccountFill: PerpsAccountFill{
				TradeID:            f.TradeID,
				OrderID:            f.OrderID,
				InstrumentID:       f.InstrumentID,
				Side:               f.Side,
				Settlement:         f.Settlement,
				Price:              f.Price,
				Quantity:           f.Quantity,
				Taker:              f.Taker,
				Fee:                f.Fee,
				BuilderFee:         f.BuilderFee,
				TotalFee:           f.TotalFee,
				Builder:            f.Builder,
				FeeAsset:           f.FeeAsset,
				PreviousSize:       f.PreviousSize,
				PreviousEntryPrice: f.PreviousEntryPrice,
				PnL:                f.PnL,
				Liquidation:        f.Liquidation,
				Timestamp:          f.Timestamp,
			},
			ClientOrderID: f.ClientID,
		}
	}
	return out, nil
}

func (e PerpsSessionEvent) AsFunding() (*PerpsAccountFundingPayment, error) {
	wire, err := decodeEvent[struct {
		ID        int64  `json:"id"`
		IID       int    `json:"iid"`
		Size      string `json:"sz"`
		Rate      string `json:"fr"`
		Funding   string `json:"fund"`
		Asset     string `json:"fua"`
		Timestamp int64  `json:"ts"`
	}](e, "funding")
	if err != nil {
		return nil, err
	}
	return &PerpsAccountFundingPayment{
		ID:           wire.ID,
		InstrumentID: wire.IID,
		Size:         wire.Size,
		FundingRate:  wire.Rate,
		FundingAsset: wire.Asset,
		Funding:      wire.Funding,
		Timestamp:    wire.Timestamp,
	}, nil
}

type PerpsDepositUpdate struct {
	Hash   string             `json:"hash"`
	Asset  string             `json:"asset"`
	Amount string             `json:"amount"`
	Status PerpsDepositStatus `json:"status"`
}
type PerpsWithdrawalUpdate struct {
	ID     int                   `json:"withdraw_id"`
	Asset  string                `json:"asset"`
	Amount string                `json:"amount"`
	Fee    string                `json:"fee"`
	Status PerpsWithdrawalStatus `json:"status"`
	To     string                `json:"to"`
	Hash   string                `json:"hash"`
}

func (e PerpsSessionEvent) AsDeposit() (*PerpsDepositUpdate, error) {
	return decodeEvent[PerpsDepositUpdate](e, "deposits")
}

func (e PerpsSessionEvent) AsWithdrawal() (*PerpsWithdrawalUpdate, error) {
	return decodeEvent[PerpsWithdrawalUpdate](e, "withdrawals")
}

type PerpsTPSLUpdate struct {
	OrderID int    `json:"oid"`
	Status  string `json:"st"`
	Reason  string `json:"reason,omitempty"`
}

func (e PerpsSessionEvent) AsTPSL() (*PerpsTPSLUpdate, error) {
	if !strings.HasPrefix(e.Channel, "tpsl::") {
		return nil, fmt.Errorf("perps: event is not a TP/SL update")
	}
	return decodeEvent[PerpsTPSLUpdate](e, e.Channel)
}
