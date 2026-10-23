package data

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestGetTokenReferences(t *testing.T) {
	var query [][2]string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/tokens" {
			t.Errorf("path = %s", r.URL.Path)
		}
		query = nil
		for key, values := range r.URL.Query() {
			for _, value := range values {
				query = append(query, [2]string{key, value})
			}
		}
		_, _ = io.WriteString(w, `{"data":[{
			"token_id":"18446744073709551616",
			"condition_id":"0x`+strings.Repeat("ab", 32)+`",
			"structural_condition_id":"0x`+strings.Repeat("cd", 31)+`00",
			"module":"neg_risk",
			"outcome_index":1,
			"clob_index":0,
			"outcome":"Yes",
			"opposite_token_id":"18446744073709551617",
			"resolved":false,
			"final_price":0.125,
			"title":null,
			"market_slug":"market",
			"closed":false,
			"neg_risk":true,
			"neg_risk_market_id":"0x`+strings.Repeat("ef", 32)+`",
			"question_index":3,
			"event_id":"42",
			"event_slug":"event"
		}]}`)
	})
	references, err := client.GetTokenReferences(t.Context(), TokenReferenceSelectors{
		AssetIDs: []string{" 00018446744073709551616 ", "", "18446744073709551616"},
	})
	if err != nil {
		t.Fatalf("GetTokenReferences: %v", err)
	}
	want := [][2]string{{"token_id", "18446744073709551616"}}
	if !reflect.DeepEqual(query, want) {
		t.Fatalf("query = %v, want %v", query, want)
	}
	reference := references[0]
	if reference.AssetID != "18446744073709551616" ||
		reference.StructuralConditionID == nil ||
		reference.Module != TokenModuleNegRisk ||
		reference.OutcomeIndex != 1 ||
		reference.CLOBIndex == nil || *reference.CLOBIndex != 0 ||
		reference.OppositeAssetID == nil ||
		reference.Resolved ||
		reference.FinalPrice == nil || reference.FinalPrice.String() != "0.125" ||
		reference.Title != nil ||
		reference.Closed == nil || *reference.Closed ||
		reference.NegRisk == nil || !*reference.NegRisk ||
		reference.QuestionIndex == nil || *reference.QuestionIndex != 3 ||
		reference.EventID == nil || *reference.EventID != "42" {
		t.Fatalf("reference lost wire values: %+v", reference)
	}
}

func TestGetTokenReferencesByCondition(t *testing.T) {
	var conditions []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		conditions = r.URL.Query()["condition"]
		_, _ = io.WriteString(w, `{"data":[]}`)
	})
	// The 31-byte form keeps its width; duplicates and case collapse.
	_, err := client.GetTokenReferences(t.Context(), TokenReferenceSelectors{
		ConditionIDs: []string{
			"0x" + strings.Repeat("AB", 31),
			"0x" + strings.Repeat("ab", 31),
			"0x" + strings.Repeat("cd", 32),
		},
	})
	if err != nil {
		t.Fatalf("GetTokenReferences: %v", err)
	}
	want := []string{"0x" + strings.Repeat("ab", 31), "0x" + strings.Repeat("cd", 32)}
	if !slices.Equal(conditions, want) {
		t.Fatalf("condition = %v, want %v", conditions, want)
	}
}

func TestGetTokenReferencesValidation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid selectors reached the service: %s", r.URL.RawQuery)
	})
	tooManyAssets := make([]string, 51)
	for i := range tooManyAssets {
		tooManyAssets[i] = strconv.Itoa(i + 1)
	}
	tooManyConditions := make([]string, 11)
	for i := range tooManyConditions {
		tooManyConditions[i] = fmt.Sprintf("0x%062x", i+1)
	}
	for name, selectors := range map[string]TokenReferenceSelectors{
		"no selectors":          {},
		"both families":         {AssetIDs: []string{"1"}, ConditionIDs: []string{"0x" + strings.Repeat("ab", 32)}},
		"non-decimal asset":     {AssetIDs: []string{"0x1"}},
		"79-digit asset":        {AssetIDs: []string{strings.Repeat("1", 79)}},
		"empty asset":           {AssetIDs: []string{" "}},
		"too many assets":       {AssetIDs: tooManyAssets},
		"short condition":       {ConditionIDs: []string{"0x" + strings.Repeat("ab", 30)}},
		"unprefixed":            {ConditionIDs: []string{strings.Repeat("ab", 32)}},
		"non-hex condition":     {ConditionIDs: []string{"0x" + strings.Repeat("zz", 32)}},
		"too many conditions":   {ConditionIDs: tooManyConditions},
		"oversized condition":   {ConditionIDs: []string{"0x" + strings.Repeat("ab", 33)}},
		"empty condition set":   {ConditionIDs: []string{"", " "}},
		"whitespace-only asset": {AssetIDs: []string{"\t"}},
	} {
		if _, err := client.GetTokenReferences(t.Context(), selectors); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}
