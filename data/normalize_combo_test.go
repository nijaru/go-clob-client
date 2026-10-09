package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const (
	comboPositionJSON  = `{"combo_condition_id":%s,"combo_position_id":"position-01","proxy_wallet":"wallet","outcome_index":1,"outcome_label":"No","current_size":1,"entry_avg_price_usdc":0.5,"entry_cost_usdc":0.5,"gross_entry_cost_usdc":0.5,"entry_fees_usdc":0,"realized_payout_usdc":0,"status":"OPEN","redeemable":false,"first_entry_at":1785425912,"updated_at":1785425912,"legs_total":0,"legs_resolved":0,"legs_pending":0,"legs":[]}`
	comboTradeJSON     = `{"type":"TRADE","proxy_wallet":"wallet","timestamp":1785425912,"transaction_hash":"0x01","condition_id":%s,"token_id":"position-01","is_combo":true,"side":"BUY","size":3.1,"usdc_size":1.55,"price":0.5}`
	comboLifecycleJSON = `{"id":"operation","type":"SPLIT","proxy_wallet":"wallet","combo_condition_id":%s,"combo_position_id":"position-01","timestamp":1785425912,"transaction_hash":"0x01","block_number":1,"legs":[]}`
	comboWinnerJSON    = `{"kind":"combo","position_id":"position-01","win_rank":2,"user_id":"wallet","condition_id":%s,"pnl":"15.000000000000000001","initial_value":10,"final_value":25,"resolved_at":1785425912}`
)

func TestComboResponseConditionIDs(t *testing.T) {
	models := []struct {
		name, template string
		decode         func([]byte) (string, error)
	}{
		{"position", comboPositionJSON, func(raw []byte) (string, error) {
			var value ComboPosition
			err := json.Unmarshal(raw, &value)
			return value.ConditionID, err
		}},
		{"trade activity", comboTradeJSON, func(raw []byte) (string, error) {
			var value Activity
			err := json.Unmarshal(raw, &value)
			if value.ConditionID == nil {
				return "", err
			}
			return *value.ConditionID, err
		}},
		{"lifecycle", comboLifecycleJSON, func(raw []byte) (string, error) {
			var value ComboActivity
			err := json.Unmarshal(raw, &value)
			return value.ConditionID, err
		}},
		{"winner", comboWinnerJSON, func(raw []byte) (string, error) {
			var value BiggestWinner
			err := json.Unmarshal(raw, &value)
			return value.ConditionID, err
		}},
	}
	canonical := "0x03" + strings.Repeat("ab", 30)
	for _, model := range models {
		t.Run(model.name, func(t *testing.T) {
			for _, id := range []string{canonical, "0x03" + strings.Repeat("AB", 30), canonical + "00", canonical + "01"} {
				got, err := model.decode([]byte(fmt.Sprintf(model.template, fmt.Sprintf("%q", id))))
				if err != nil || got != canonical {
					t.Fatalf("%s: got %q error=%v", id, got, err)
				}
			}
			for _, rawID := range []string{
				`null`, `1`, `""`, `"0x03"`, fmt.Sprintf("%q", canonical+"02"),
				fmt.Sprintf("%q", strings.Replace(canonical, "03", "02", 1)),
				fmt.Sprintf("%q", canonical[:63]+"g"), fmt.Sprintf("%q", canonical+"0000"),
			} {
				if _, err := model.decode([]byte(fmt.Sprintf(model.template, rawID))); err == nil {
					t.Fatalf("accepted invalid combo ID %s", rawID)
				}
			}
		})
	}
}

func TestComboNormalizationDoesNotInferMarketOrUnknownTypes(t *testing.T) {
	marketID := "0x03" + strings.Repeat("AB", 30) + "01"
	tradeRaw := strings.Replace(
		fmt.Sprintf(comboTradeJSON, fmt.Sprintf("%q", marketID)),
		`"is_combo":true`,
		`"is_combo":false`,
		1,
	)
	var activity Activity
	if err := json.Unmarshal([]byte(tradeRaw), &activity); err != nil ||
		activity.ConditionID == nil ||
		*activity.ConditionID != marketID {
		t.Fatalf("market trade ID changed: %+v error=%v", activity, err)
	}
	winnerRaw := strings.Replace(
		fmt.Sprintf(comboWinnerJSON, fmt.Sprintf("%q", marketID)),
		`"kind":"combo"`,
		`"kind":"market","event_id":1`,
		1,
	)
	var winner BiggestWinner
	if err := json.Unmarshal([]byte(winnerRaw), &winner); err != nil ||
		winner.ConditionID != marketID {
		t.Fatalf("market winner ID changed: %+v error=%v", winner, err)
	}
	unknown := `{"type":"FUTURE","is_combo":true,"condition_id":{"future":"shape"},"timestamp":1785425912}`
	if err := json.Unmarshal([]byte(unknown), &activity); err != nil ||
		string(activity.Raw) != unknown ||
		activity.ConditionID != nil {
		t.Fatalf("unknown activity lost raw payload: %+v error=%v", activity, err)
	}
}

func TestInvalidComboResponseHasServiceErrorType(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(
			w,
			`{"data":[%s],"pagination":{"has_more":false,"next_cursor":null}}`,
			fmt.Sprintf(comboPositionJSON, `"0x03"`),
		)
	})
	_, err := client.GetComboPositions(t.Context(), ComboPositionsParams{User: "wallet"})
	var inputErr *InputError
	if !errors.Is(err, ErrInvalidResponse) || errors.As(err, &inputErr) {
		t.Fatalf("server's malformed combo was not a response error: %v", err)
	}
}
