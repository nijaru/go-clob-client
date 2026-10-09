package sports

import (
	"bytes"
	"encoding/json"
	"os"
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

func TestUpstreamFixtures(t *testing.T) {
	for _, name := range []string{"python-game", "ts-minimal"} {
		t.Run(name, func(t *testing.T) {
			wire := fixture(t, name)
			event, err := ParseEvent(wire)
			if err != nil {
				t.Fatal(err)
			}
			if event.GameID != 123 || event.LeagueAbbreviation != "NBA" || !event.Live ||
				event.Ended {
				t.Fatalf("wrong required fields: %+v", event)
			}
			if name == "python-game" {
				for key, pair := range map[string][2]*string{
					"sportradar": {event.SportradarGameID, ptr("sr-abc")},
					"slug":       {event.Slug, ptr("lakers-vs-celtics")},
					"home":       {event.HomeTeam, ptr("LAL")},
					"away":       {event.AwayTeam, ptr("BOS")},
					"period":     {event.Period, ptr("Q4")},
					"elapsed":    {event.Elapsed, ptr("10:32")},
					"turn":       {event.Turn, ptr("BOS")},
				} {
					if pair[0] == nil || *pair[0] != *pair[1] {
						t.Errorf("lost %s", key)
					}
				}
				if event.Status != "live" || event.Score != "98-102" {
					t.Fatalf("lost opaque status/score: %+v", event)
				}
			} else if event.HomeTeam != nil || event.Turn != nil || event.Status != "inprogress" {
				t.Fatalf("wrong minimal event: %+v", event)
			}
			if !bytes.Equal(event.Raw, wire) {
				t.Fatal("raw wire changed")
			}
			wire[0] = 'x'
			if event.Raw[0] != '{' {
				t.Fatal("raw wire aliases caller memory")
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestMetadataAndTimestampRepresentations(t *testing.T) {
	// Timestamp values are from the Python upstream timestamp cases. Unlike the
	// unified SDK's envelope, the Go event retains BOTH original aliases.
	for _, timestamp := range []string{`"1710000000000"`, `1710000000000`, `"2024-03-09T12:00:00+00:00"`} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(fixture(t, "python-game"), &fields); err != nil {
			t.Fatal(err)
		}
		fields["finishedTimestamp"] = json.RawMessage(`null`)
		fields["finished_timestamp"] = json.RawMessage(timestamp)
		fields["providerMetadata"] = json.RawMessage(
			`{"sequence":9007199254740993,"future":[true,null]}`,
		)
		fields["homeTeam"] = json.RawMessage(`""`)
		fields["awayTeam"] = json.RawMessage(`null`)
		wire, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		event, err := ParseEvent(wire)
		if err != nil {
			t.Fatal(err)
		}
		if string(event.FinishedTimestamp) != "null" ||
			string(event.FinishedTimestampSnake) != timestamp {
			t.Fatal("timestamp aliases or representation changed")
		}
		if event.HomeTeam == nil || *event.HomeTeam != "" || event.AwayTeam != nil {
			t.Fatal("empty/null optional fields changed")
		}
		if !bytes.Equal(wire, event.Raw) {
			t.Fatal("metadata lost or rounded")
		}
	}
}

func TestRejectMalformedAndUnrelatedEvents(t *testing.T) {
	for _, wire := range []string{"null", "[]", "{", `{"topic":"sports","payload":{}}`} {
		if _, err := ParseEvent([]byte(wire)); err == nil {
			t.Errorf("accepted %s", wire)
		}
	}
	for key, wrong := range map[string]string{
		"gameId": `"123"`, "leagueAbbreviation": `false`, "status": `[]`,
		"live": `"true"`, "ended": `0`, "score": `{}`, "turn": `3`,
		"finishedTimestamp": `true`, "finished_timestamp": `{}`,
	} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(fixture(t, "ts-minimal"), &fields); err != nil {
			t.Fatal(err)
		}
		fields[key] = json.RawMessage(wrong)
		wire, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseEvent(wire); err == nil {
			t.Errorf("accepted wrong type for %s", key)
		}
	}
	for _, key := range []string{"gameId", "leagueAbbreviation", "status", "live", "ended", "score"} {
		for _, missing := range []bool{true, false} {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(fixture(t, "ts-minimal"), &fields); err != nil {
				t.Fatal(err)
			}
			if missing {
				delete(fields, key)
			} else {
				fields[key] = json.RawMessage(`null`)
			}
			wire, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseEvent(wire); err == nil {
				t.Errorf("accepted missing/null %s", key)
			}
		}
	}
}
