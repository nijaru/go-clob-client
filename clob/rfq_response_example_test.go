package clob_test

import (
	"encoding/json"
	"fmt"

	"github.com/nijaru/go-clob-client/clob"
)

func ExampleComboMarket_ParsedOutcomes() {
	// Catalogs may encode their arrays as JSON strings. Financial scalars remain
	// exact text; use Dec only when local arithmetic is needed and supported.
	const wire = `{"id":123,"outcomes":"[\"Yes\",\"No\"]","position_ids":["yes-position","no-position"],"outcome_prices":[0.1234567890123456789,"0.9"],"volume":9007199254740993.001}`
	var market clob.ComboMarket
	if err := json.Unmarshal([]byte(wire), &market); err != nil {
		panic(err)
	}
	outcomes, err := market.ParsedOutcomes()
	if err != nil {
		panic(err)
	}
	fmt.Println(market.ID, market.Volume)
	fmt.Println(outcomes.Yes.Label, outcomes.Yes.Price)
	// Output:
	// 123 9007199254740993.001
	// Yes 0.1234567890123456789
}

func ExampleCollateralReturnPlan() {
	const wire = `{"block_number":9007199254740993,"estimated_cost":0.1234567890123456789,"net_pusd_out":"1.000001","operations":[{"kind":"merge_on_event","event_id":42,"amount":"1000001"}]}`
	var plan clob.CollateralReturnPlan
	if err := json.Unmarshal([]byte(wire), &plan); err != nil {
		panic(err)
	}
	// Plan balance fields are human-readable decimals. Operation and position
	// amounts deliberately retain the service's e6 integer units.
	fmt.Println(plan.BlockNumber, plan.EstimatedCost)
	fmt.Println(plan.NetPUSDOut, plan.Operations[0].Amount)
	// Output:
	// 9007199254740993 0.1234567890123456789
	// 1.000001 1000001
}
