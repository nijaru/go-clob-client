package data

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	jsonv2 "github.com/go-json-experiment/json"
)

func TestExactDecimalAndTimestamp(t *testing.T) {
	for _, raw := range []string{`"12345678901234567890.123456789012345678901"`, `12345678901234567890.123456789012345678901`} {
		var amount DecimalString
		if err := jsonv2.Unmarshal([]byte(raw), &amount); err != nil {
			t.Fatal(err)
		}
		if string(amount) != "12345678901234567890.123456789012345678901" {
			t.Fatalf("amount rounded: %s", amount)
		}
	}
	for _, raw := range []string{`"NaN"`, `null`, `"1/3"`} {
		var amount DecimalString
		if err := jsonv2.Unmarshal([]byte(raw), &amount); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{`"2026-07-30T15:38:32.234415Z"`, `1785425912.234415`, `"1785425912.234415"`} {
		var stamp Timestamp
		if err := jsonv2.Unmarshal([]byte(raw), &stamp); err != nil {
			t.Fatal(err)
		}
		if stamp.Unix() != 1785425912 || stamp.Nanosecond() != 234415000 ||
			stamp.Location() != time.UTC {
			t.Fatalf("timestamp lost units/precision: %v", stamp)
		}
	}
}

func TestV2PositionWireNames(t *testing.T) {
	// The server returns snake_case, a large asset identifier, and optional
	// epoch-second timestamps; fixtures marshaled from our own types miss this.
	raw := `{"proxy_wallet":"0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b","token_id":"94476829201604408463453426454480212459887267917122244941405244686637914508323","condition_id":"0x7ad403c3508f8e3912940fd1a913f227591145ca0614074208e0b962d5fcc422","current_size":400000,"avg_price":0.5,"entry_cost_usdc":200000,"entry_fees_usdc":0,"total_cost_usdc":200000,"current_price":0.7545,"current_value":301800,"total_size":10386654,"realized_pnl":0,"unrealized_pnl":101800,"total_pnl":101800,"percent_pnl":50.9,"percent_realized_pnl":-94.1886,"status":"OPEN","redeemable":false,"mergeable":true,"negative_risk":true,"archived":false,"verified":true,"event_id":31552,"last_event_at":1787499970,"first_entry_at":null,"end_date":"1970-01-01"}`
	var position Position
	if err := jsonv2.Unmarshal([]byte(raw), &position); err != nil {
		t.Fatal(err)
	}
	if position.Wallet == "" || len(position.AssetID) != 77 || position.EntryCostUSDC != "200000" ||
		position.EntryFeesUSDC != "0" ||
		position.Status != PositionStatusOpen ||
		position.EventID == nil ||
		*position.EventID != "31552" ||
		position.LastEventAt == nil ||
		position.LastEventAt.Unix() != 1787499970 ||
		position.FirstEntryAt != nil || position.EndDate != nil {
		t.Fatalf("incorrect v2 position: %+v", position)
	}
}

func TestResolutionUnitsAndUnsetSentinel(t *testing.T) {
	raw := `{"status":"resolved","extended_review":false,"was_disputed":false,"new_version_q":true,"last_update_timestamp":"1785355919","log_index":"170","transaction_hash":"","reproposed_price":"69","price":"0","payouts":[1000000,"0.000000001"]}`
	var resolution Resolution
	if err := jsonv2.Unmarshal([]byte(raw), &resolution); err != nil {
		t.Fatal(err)
	}
	if resolution.LogIndex == nil || *resolution.LogIndex != 170 ||
		resolution.TransactionHash != nil ||
		resolution.LastUpdatedAt.Unix() != 1785355919 ||
		resolution.ReproposedPrice != nil ||
		resolution.Price == nil ||
		*resolution.Price != "0" ||
		resolution.Payouts == nil ||
		resolution.Payouts[0] != "1" ||
		resolution.Payouts[1] != "0.000000000000001" {
		t.Fatalf("resolution units/sentinels incorrect: %+v", resolution)
	}
	if err := jsonv2.Unmarshal([]byte(`{"payouts":[1000000]}`), &resolution); err == nil {
		t.Fatal("accepted a malformed payout pair")
	}
}

func TestActivityAndWinnerKinds(t *testing.T) {
	var activity Activity
	raw := `{"type":"TRADE","proxy_wallet":"0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b","timestamp":1785425912,"transaction_hash":"0x01","condition_id":"0x03abababababababababababababababababababababababababababababab","token_id":"42","is_combo":true,"side":"BUY","size":3.1,"usdc_size":1.55,"price":0.5}`
	if err := jsonv2.Unmarshal([]byte(raw), &activity); err != nil {
		t.Fatal(err)
	}
	if !activity.IsCombo || activity.Amount == nil || *activity.Amount != "1.55" ||
		activity.Shares == nil ||
		*activity.Shares != "3.1" ||
		activity.Raw != nil {
		t.Fatalf("lost combo trade fields: %+v", activity)
	}
	raw = `{"type":"FUTURE_CREDIT","name":"","timestamp":1785425912,"new_field":{"reason":"test"},"price":{"unit":"USD","value":"1"},"size":[1,2],"outcome_index":{"new":"shape"}}`
	if err := jsonv2.Unmarshal([]byte(raw), &activity); err != nil {
		t.Fatal(err)
	}
	if string(activity.Raw) != raw || activity.Type != "FUTURE_CREDIT" ||
		activity.Timestamp.IsZero() ||
		activity.Name != nil ||
		activity.Price != nil {
		t.Fatal("unknown wallet event or its common envelope was discarded")
	}
	if err := jsonv2.Unmarshal([]byte(`{"type":"TRADE"}`), &activity); err == nil {
		t.Fatal("accepted an incomplete known trade")
	}
	var winner BiggestWinner
	if err := jsonv2.Unmarshal([]byte(`{"kind":"combo","position_id":"42","win_rank":2,"user_id":"0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b","condition_id":"0x030b98f62eadaee25b8c04abe48fb41d480000000000000000000000000000","pnl":"15.000000000000000001","initial_value":10,"final_value":25,"resolved_at":1785425912}`), &winner); err != nil {
		t.Fatal(err)
	}
	if winner.Kind != BiggestWinnerKindCombo || winner.AssetID != "42" || winner.Rank != 2 ||
		winner.PnL != "15.000000000000000001" {
		t.Fatalf("lost combo winner: %+v", winner)
	}
	if err := jsonv2.Unmarshal([]byte(`{"kind":"market","position_id":"42"}`), &winner); err == nil {
		t.Fatal("accepted market winner without an event")
	}
}

func TestIndexedApprovalValidation(t *testing.T) {
	address := "0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b"
	prefix := `{"token":"` + address + `","spender":"` + address + `","standard":"ERC20",`
	var row ApprovalContract
	if err := jsonv2.Unmarshal([]byte(prefix+`"amount":"max","approved":true}`), &row); err != nil {
		t.Fatal(err)
	}
	if row.Amount == nil || *row.Amount != ApprovalMax || !row.Approved || row.Raw != nil {
		t.Fatalf("incorrect known approval: %+v", row)
	}
	for _, tail := range []string{`"approved":true}`, `"amount":"max"}`, `"amount":"01","approved":true}`, `"amount":"` + strings.Repeat("9", 78) + `","approved":true}`} {
		if err := jsonv2.Unmarshal([]byte(prefix+tail), &row); err == nil {
			t.Fatalf("malformed known row was accepted: %s", tail)
		}
	}
	unknown := `{"token":"future-chain-token","spender":"future-spender","standard":"FUTURE","amount":{"new":"schema"}}`
	if err := jsonv2.Unmarshal([]byte(unknown), &row); err != nil {
		t.Fatal(err)
	}
	if row.Approved || !json.Valid(row.Raw) || string(row.Raw) != unknown {
		t.Fatal("unknown approval was promoted or lost")
	}
}
