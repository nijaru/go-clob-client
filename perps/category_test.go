package perps

import (
	"encoding/json"
	"testing"
)

func TestInstrumentCategoryPreservesKnownAndFutureTypes(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"pre_ipo", "future_category"} {
		var instrument PerpsInstrument
		raw := []byte(`{"instrument_id":2,"category":"` + name + `"}`)
		if err := json.Unmarshal(raw, &instrument); err != nil {
			t.Fatal(err)
		}
		if string(instrument.Category) != name {
			t.Fatalf("category = %q, want %q", instrument.Category, name)
		}
	}
	if PerpsCategoryPreIPO != "pre_ipo" {
		t.Fatalf("pre-IPO category wire value: %q", PerpsCategoryPreIPO)
	}
}
