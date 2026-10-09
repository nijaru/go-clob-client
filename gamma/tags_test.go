package gamma

import (
	"fmt"
	"net/http"
	"testing"
)

func TestClient_GetTag(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Tag{ID: "tag-1", Label: ptr("Politics")})
	})

	tag, err := client.GetTag(t.Context(), "tag-1")
	if err != nil {
		t.Fatalf("GetTag: %v", err)
	}
	if tag.Label == nil || *tag.Label != "Politics" {
		t.Errorf("label = %v", tag.Label)
	}
}

func TestClient_GetTagBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/slug/politics" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Tag{ID: "tag-1", Slug: ptr("politics")})
	})

	tag, err := client.GetTagBySlug(t.Context(), "politics")
	if err != nil {
		t.Fatalf("GetTagBySlug: %v", err)
	}
	if tag.Slug == nil || *tag.Slug != "politics" {
		t.Errorf("slug = %v", tag.Slug)
	}
}

func TestClient_GetRelatedTags(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1/related-tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []RelatedTag{{ID: "rt-1", TagID: "tag-1", Rank: ptr(1)}})
	})

	tags, err := client.GetRelatedTags(t.Context(), "tag-1")
	if err != nil {
		t.Fatalf("GetRelatedTags: %v", err)
	}
	if len(tags) != 1 || (tags[0].Rank == nil || *tags[0].Rank != 1) {
		t.Errorf("tags = %+v", tags)
	}
}

func TestClient_GetRelatedTagsBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/slug/politics/related-tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []RelatedTag{{ID: "rt-1"}})
	})

	tags, err := client.GetRelatedTagsBySlug(t.Context(), "politics")
	if err != nil {
		t.Fatalf("GetRelatedTagsBySlug: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetTagsRelatedToTag(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1/related-tags/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1", Label: ptr("Elections")}})
	})

	tags, err := client.GetTagsRelatedToTag(t.Context(), "tag-1")
	if err != nil {
		t.Fatalf("GetTagsRelatedToTag: %v", err)
	}
	if len(tags) != 1 || (tags[0].Label == nil || *tags[0].Label != "Elections") {
		t.Errorf("tags = %+v", tags)
	}
}

func TestClient_GetTagsRelatedToTagBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/slug/politics/related-tags/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1"}})
	})

	tags, err := client.GetTagsRelatedToTagBySlug(t.Context(), "politics")
	if err != nil {
		t.Fatalf("GetTagsRelatedToTagBySlug: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetRelatedTagResources(t *testing.T) {
	omitEmpty := true
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1/related-tags/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("locale") != "en" || q.Get("status") != "active" || q.Get("omit_empty") != "true" {
			t.Errorf("query = %v", q)
		}
		writeJSON(w, []Tag{{ID: "market-1"}})
	})

	tags, err := client.GetRelatedTagResources(t.Context(), "tag-1", RelatedTagResourceParams{
		Locale: "en", OmitEmpty: &omitEmpty, Status: "active",
	})
	if err != nil {
		t.Fatalf("GetRelatedTagResources: %v", err)
	}
	if len(tags) != 1 || tags[0].ID != "market-1" {
		t.Errorf("tags = %+v", tags)
	}
}

func TestClient_GetTags(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1"}, {ID: "t-2"}})
	})

	tags, err := client.GetTags(t.Context())
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetTagsPageClampsAndFilters(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit = %q, want 100", got)
		}
		if got := r.URL.Query().Get("offset"); got != "7" {
			t.Errorf("offset = %q, want 7", got)
		}
		checks := map[string]string{
			"ascending":        "true",
			"include_template": "true",
			"is_carousel":      "false",
			"locale":           "en",
			"order":            "label",
		}
		for key, want := range checks {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		writeJSON(w, []Tag{{ID: "tag-1"}})
	})

	tags, err := client.GetTagsPage(t.Context(), TagFilterParams{
		Ascending:       boolPtr(true),
		IncludeTemplate: boolPtr(true),
		IsCarousel:      boolPtr(false),
		Locale:          "en",
		Order:           "label",
		Limit:           150,
		Offset:          7,
	})
	if err != nil {
		t.Fatalf("GetTagsPage: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("got %d tags, want 1", len(tags))
	}
}

func TestClient_IterTagsClampsOfficialLimit(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("call %d limit = %q, want 100", call, got)
		}
		switch call {
		case 1:
			tags := make([]Tag, 100)
			for i := range tags {
				tags[i] = Tag{ID: fmt.Sprintf("tag-%d", i)}
			}
			writeJSON(w, tags)
		case 2:
			if got := r.URL.Query().Get("offset"); got != "100" {
				t.Errorf("second offset = %q, want 100", got)
			}
			writeJSON(w, []Tag{{ID: "tag-100"}})
		default:
			t.Errorf("iterator made unexpected call %d", call)
		}
	})

	var ids []string
	for tag, err := range client.IterTags(t.Context(), TagFilterParams{Limit: 150}) {
		if err != nil {
			t.Fatalf("IterTags: %v", err)
		}
		ids = append(ids, tag.ID)
	}
	if len(ids) != 101 || ids[100] != "tag-100" {
		t.Fatalf("collected %d tags, last %q", len(ids), ids[len(ids)-1])
	}
}
