package gamma

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestCommentNumericParentID(t *testing.T) {
	// py-sdk ed8d04ca test_gamma_models.py::test_comment_normalizes_flat_payload.
	var comment Comment
	if err := json.Unmarshal([]byte(`{"id":"12345","parentEntityID":123,"parentCommentID":67890}`), &comment); err != nil {
		t.Fatal(err)
	}
	if comment.ParentEntityID != "123" || comment.ParentCommentID == nil ||
		*comment.ParentCommentID != "67890" {
		t.Fatalf("thread identity: %+v", comment)
	}
}

func TestSearchAndNestedMetadataUnion(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// TS/Python event_count, Tag.templates and Collection.disqusThread;
		// the aliases/fields cannot be tested by encoding the Go model itself.
		fmt.Fprint(
			w,
			`{"tags":[{"id":"1","event_count":8}],"events":[{"id":"2","tags":[{"id":"1","templates":[{"id":"t","displayName":"Template"}]}],"collections":[{"id":"c","disqusThread":"thread"}]}]}`,
		)
	})
	result, err := client.Search(t.Context(), SearchParams{Query: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Tags[0].EventCount == nil || *result.Tags[0].EventCount != 8 ||
		len(result.Events[0].Tags[0].Templates) != 1 ||
		*result.Events[0].Tags[0].Templates[0].DisplayName != "Template" ||
		*result.Events[0].Collections[0].DisqusThread != "thread" {
		t.Fatalf("lost discovery metadata: %+v", result)
	}
	var tag SearchTag
	if err := json.Unmarshal([]byte(`{"event_count":9,"eventCount":0}`), &tag); err != nil {
		t.Fatal(err)
	}
	if tag.EventCount == nil || *tag.EventCount != 0 {
		t.Fatalf("camelCase count lost: %+v", tag)
	}
}

func TestCreatorURLPriority(t *testing.T) {
	// TS normalizeEvent uses creatorUrl ?? creatorURL, never last-key-wins.
	for _, payload := range []string{
		`{"creatorUrl":"new","creatorURL":null}`,
		`{"creatorURL":null,"creatorUrl":"new"}`,
		`{"creatorUrl":"new","creatorURL":"old"}`,
		`{"creatorURL":"old","creatorUrl":"new"}`,
		`{"creatorUrl":null,"creatorURL":"new"}`,
	} {
		var creator EventCreator
		if err := json.Unmarshal([]byte(payload), &creator); err != nil {
			t.Fatal(err)
		}
		if creator.CreatorURL == nil || *creator.CreatorURL != "new" {
			t.Fatalf("alias priority %s: %+v", payload, creator)
		}
	}
}

func TestOptionalDecimalAndLegacyText(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte(`{"id":"1","volume":"","liquidity":null,"estimatedValue":"unknown"}`), &event); err != nil {
		t.Fatal(err)
	}
	if event.Volume != "" || event.Liquidity != "" || event.EstimatedValue != "unknown" {
		t.Fatalf("optional estimates: %+v", event)
	}
	var market Market
	if err := json.Unmarshal([]byte(`{"id":"1","lowerBound":"","upperBound":"unbounded","groupItemThreshold":"TBD","umaBond":"pending","denominationToken":9007199254740993}`), &market); err != nil {
		t.Fatal(err)
	}
	if market.UpperBound != "unbounded" || market.GroupItemThreshold != "TBD" ||
		market.UmaBond != "pending" ||
		market.DenominationToken == nil ||
		*market.DenominationToken != "9007199254740993" {
		t.Fatalf("legacy text: %+v", market)
	}
	var position CommentPosition
	if err := json.Unmarshal([]byte(`{"tokenId":9007199254740993,"positionSize":"0"}`), &position); err != nil {
		t.Fatal(err)
	}
	if position.TokenID == nil || *position.TokenID != "9007199254740993" {
		t.Fatalf("token ID rounded: %+v", position)
	}
	if err := json.Unmarshal([]byte(`{"estimatedValue":9007199254740993.125}`), &event); err != nil {
		t.Fatal(err)
	}
	if event.EstimatedValue != "9007199254740993.125" {
		t.Fatalf("estimate rounded: %s", event.EstimatedValue)
	}
}
