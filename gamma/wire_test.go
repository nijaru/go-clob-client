package gamma

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// These payloads are copied from upstream tests, not encoded from Go models:
// rs-clob-client-v2 561830b tests/gamma.rs::sports_should_succeed;
// py-sdk ed8d04ca tests/unit/test_gamma_models.py::test_{market,event}_normalizes_groups_from_flat_payload.
func TestPinnedWireFixtures(t *testing.T) {
	t.Run("sports", func(t *testing.T) {
		data := fixture(t, "rust-sports")
		_, client := newTestServer(
			t,
			func(w http.ResponseWriter, r *http.Request) { w.Write(data) },
		)
		sports, err := client.GetSports(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(sports) != 1 || sports[0].Sport != "ncaab" || sports[0].Name != nil ||
			!slices.Equal(sports[0].Tags, []string{"1", "2", "3"}) {
			t.Fatalf("sports: %+v", sports)
		}
	})
	t.Run("market", func(t *testing.T) {
		data := fixture(t, "py-market")
		_, client := newTestServer(
			t,
			func(w http.ResponseWriter, r *http.Request) { w.Write(data) },
		)
		m, err := client.GetMarket(t.Context(), "MARKET-1")
		if err != nil {
			t.Fatal(err)
		}
		if m.Volume != "100" || m.Volume24h != "20" || m.OrderMinSize != "5" ||
			m.OrderPriceMinTickSize != "0.01" ||
			m.Spread != "0.02" ||
			m.RewardsMinSize != "10" ||
			m.FeeSchedule == nil ||
			m.FeeSchedule.Rate != "0.01" ||
			m.FeeSchedule.TakerOnly == nil || *m.FeeSchedule.TakerOnly ||
			m.PositionIDs[0] != "POSITION-YES" ||
			m.Events[0].ID != "EVENT-1" ||
			m.Tags[0].ID != "TAG-1" {
			t.Fatalf("market: %+v", m)
		}
	})
	t.Run("event", func(t *testing.T) {
		data := fixture(t, "py-event")
		_, client := newTestServer(
			t,
			func(w http.ResponseWriter, r *http.Request) { w.Write(data) },
		)
		e, err := client.GetEvent(t.Context(), "EVENT-1")
		if err != nil {
			t.Fatal(err)
		}
		var metadata map[string]string
		if err := json.Unmarshal(e.EventMetadata, &metadata); err != nil {
			t.Fatal(err)
		}
		if e.Volume != "500" || e.Volume24h != "50" || e.OpenInterest != "200" ||
			(e.NegativeRisk == nil || !*e.NegativeRisk) ||
			(e.NegRiskMarketID == nil || *e.NegRiskMarketID != "NRMID") ||
			e.Sport == nil ||
			(e.Sport.Name == nil || *e.Sport.Name != "Ligue 1") ||
			!slices.Equal(e.Sport.Tags, []string{"1", "2"}) ||
			(e.Teams[0].Ordering == nil || *e.Teams[0].Ordering != "home") ||
			e.ExternalPartners[0].ID != 7 ||
			e.ExternalPartners[0].Partner.ID != 1 ||
			(e.EventCreators[0].CreatorURL == nil || *e.EventCreators[0].CreatorURL != "https://example.test/alice") ||
			e.Series[0].Volume != "1000" ||
			metadata["k"] != "v" {
			t.Fatalf("event: %+v", e)
		}
	})
}

func TestMarketNumericAndPositionWire(t *testing.T) {
	// TS 087f9443 bindings/gamma/market.ts: PositionIdArraySchema accepts direct
	// or JSON-encoded arrays; numeric shadow columns and prices are Decimalish.
	var m Market
	if err := json.Unmarshal([]byte(`{"id":"1","positionIds":"[\"yes-v2\",\"no-v2\"]","volumeNum":9007199254740993.125,"liquidityNum":"42.500","bestBid":0.45,"outcomePrices":"[0.45,0.55]","negRisk":true,"negRiskMarketID":"market-hash","negRiskRequestID":"request-hash","submitted_by":"author"}`), &m); err != nil {
		t.Fatal(err)
	}
	if m.VolumeNum != "9007199254740993.125" || m.LiquidityNum != "42.500" || m.BestBid != "0.45" ||
		(m.NegativeRisk == nil || !*m.NegativeRisk) ||
		(m.NegRiskRequestID == nil || *m.NegRiskRequestID != "request-hash") ||
		(m.SubmittedBy == nil || *m.SubmittedBy != "author") ||
		!slices.Equal(m.PositionIDs, []string{"yes-v2", "no-v2"}) {
		t.Fatalf("market: %+v", m)
	}
	rat, err := m.VolumeNum.Rat()
	if err != nil || rat.RatString() != "72057594037927945/8" {
		t.Fatalf("exact decimal: %v %v", rat, err)
	}
	for _, payload := range []string{`{"positionIds":"not-json"}`, `{"positionIds":[null,"x"]}`, `{"volumeNum":"NaN"}`, `{"volumeNum":true}`, `{"outcomePrices":["NaN"]}`} {
		if err := json.Unmarshal([]byte(payload), &m); err == nil {
			t.Fatalf("accepted invalid payload: %s", payload)
		}
	}
}

func TestTradingDecimalWire(t *testing.T) {
	// Python ed8d04ca MarketTrading accepts decimal strings and numbers.
	var m Market
	if err := json.Unmarshal([]byte(`{"minimumOrderSize":"9007199254740993.125"}`), &m); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(m.MinimumOrderSize) != "9007199254740993.125" {
		t.Fatalf("minimum order size: %v", m.MinimumOrderSize)
	}
}

func TestOptionalSportsNameAndNestedCommentMetadata(t *testing.T) {
	// TS bindings/gamma/event.ts and comment.ts, and Python models tests.
	for _, name := range []string{``, `,"name":null`, `,"name":"NBA"`} {
		var s SportsMetadata
		if err := json.Unmarshal([]byte(`{"sport":"nba","image":"i","resolution":"r","ordering":"home","tags":"1,2","series":"3"`+name+`}`), &s); err != nil {
			t.Fatal(err)
		}
		if len(s.Tags) != 2 {
			t.Fatalf("tags: %v", s.Tags)
		}
	}
	var comments []Comment
	err := json.Unmarshal(
		[]byte(
			`[{"id":"1","parentEntityID":"123","parentCommentID":null,"tradeAsset":"pos-1","media":[{"id":"m1","commentID":1,"provider":"giphy","url":"https://example.test/media"}],"profile":{"positions":[{"tokenId":"TOK-99","positionSize":42}],"profileImageOptimized":{"id":"img","relID":1,"imageUrlOptimized":"https://example.test/i"}}}]`,
		),
		&comments,
	)
	if err != nil {
		t.Fatal(err)
	}
	c := comments[0]
	if c.ParentEntityID != "123" || c.ParentCommentID != nil ||
		(c.TradeAsset == nil || *c.TradeAsset != "pos-1") ||
		(c.Media[0].Provider == nil || *c.Media[0].Provider != "giphy") ||
		(c.Profile.Positions[0].TokenID == nil || *c.Profile.Positions[0].TokenID != "TOK-99") ||
		c.Profile.Positions[0].PositionSize != "42" ||
		c.Profile.ProfileImageOptimized.RelID != "1" {
		t.Fatalf("comment: %+v", c)
	}
}

func TestPublicProfileNullAndImageOptimization(t *testing.T) {
	_, client := newTestServer(
		t,
		func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`null`)) },
	)
	p, err := client.GetPublicProfile(t.Context(), "0xabc")
	if err != nil || p != nil {
		t.Fatalf("null profile: %+v %v", p, err)
	}
	var results SearchResults
	if err := json.Unmarshal([]byte(`{"profiles":[{"id":"p1","name":null,"profileImageOptimized":{"id":"img42","imageUrlOptimized":"https://example.test/i"}}]}`), &results); err != nil {
		t.Fatal(err)
	}
	if results.Profiles[0].ProfileImageOptimized.ID == nil ||
		*results.Profiles[0].ProfileImageOptimized.ID != "img42" {
		t.Fatalf("profile: %+v", results.Profiles)
	}
}
