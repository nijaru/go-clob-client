package gamma

import (
	"encoding/json"
	"testing"
)

func TestMarketDecodesProtocolVersionAndComboStatus(t *testing.T) {
	t.Parallel()

	t.Run("v1 market with known fields", func(t *testing.T) {
		var market Market
		payload := `{
			"id": "559651",
			"version": "v1",
			"comboStatus": "enabled",
			"question": "Will it rain?",
			"conditionId": "0xabc"
		}`
		if err := json.Unmarshal([]byte(payload), &market); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if market.Version != ProtocolVersionV1 {
			t.Fatalf("version = %q, want %q", market.Version, ProtocolVersionV1)
		}
		if market.ComboStatus != ComboStatusEnabled {
			t.Fatalf("comboStatus = %q, want %q", market.ComboStatus, ComboStatusEnabled)
		}
	})

	t.Run("null and absent fields stay zero", func(t *testing.T) {
		var market Market
		payload := `{"id": "1", "version": null, "comboStatus": null}`
		if err := json.Unmarshal([]byte(payload), &market); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if market.Version != "" {
			t.Fatalf("version = %q, want empty", market.Version)
		}
		if market.ComboStatus != "" {
			t.Fatalf("comboStatus = %q, want empty", market.ComboStatus)
		}
	})

	t.Run("unknown values pass through", func(t *testing.T) {
		var market Market
		payload := `{"id": "1", "version": "v3", "comboStatus": "paused"}`
		if err := json.Unmarshal([]byte(payload), &market); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if market.Version != "v3" {
			t.Fatalf("version = %q, want passthrough v3", market.Version)
		}
		if market.ComboStatus != "paused" {
			t.Fatalf("comboStatus = %q, want passthrough paused", market.ComboStatus)
		}
	})

	t.Run("encoded array fields still normalize alongside new fields", func(t *testing.T) {
		var market Market
		payload := `{
			"id": "1",
			"version": "v2",
			"outcomes": "[\"Yes\", \"No\"]",
			"outcomePrices": "[\"0.5\", \"0.5\"]",
			"clobTokenIds": "[\"100\", \"200\"]"
		}`
		if err := json.Unmarshal([]byte(payload), &market); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if market.Version != ProtocolVersionV2 {
			t.Fatalf("version = %q, want %q", market.Version, ProtocolVersionV2)
		}
		if len(market.Outcomes) != 2 || market.Outcomes[0] != "Yes" {
			t.Fatalf("outcomes = %v", market.Outcomes)
		}
	})
}

func TestTeamDecodesOrdering(t *testing.T) {
	t.Parallel()

	var teams []Team
	payload := `[{"id": 114315, "name": "Paris Saint-Germain FC", "ordering": "home"}]`
	if err := json.Unmarshal([]byte(payload), &teams); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("teams = %d, want 1", len(teams))
	}
	if teams[0].Ordering != "home" {
		t.Fatalf("ordering = %q, want home", teams[0].Ordering)
	}
}
