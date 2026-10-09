package gamma

import (
	"fmt"
	"net/http"
	"testing"
)

func TestClient_GetComments(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("parent_entity_type") != "market" {
			t.Errorf("parent_entity_type = %q", q.Get("parent_entity_type"))
		}
		if q.Get("parent_entity_id") != "cid-1" {
			t.Errorf("parent_entity_id = %q", q.Get("parent_entity_id"))
		}
		writeJSON(w, []Comment{{ID: "c-1", Body: "hello"}})
	})

	comments, err := client.GetComments(
		t.Context(),
		CommentFilterParams{ParentEntityType: ParentEntityTypeMarket, ParentEntityID: "cid-1"},
	)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 1 || comments[0].Body != "hello" {
		t.Errorf("comments = %+v", comments)
	}
}

func TestClient_GetComment(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comments/c-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Comment{{ID: "c-1", Body: "test"}})
	})

	comments, err := client.GetComment(t.Context(), "c-1")
	if err != nil {
		t.Fatalf("GetComment: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("got %d comments", len(comments))
	}
}

func TestClient_GetCommentsByUserAddress(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comments/user_address/0xabc" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Comment{{ID: "c-1", UserAddress: "0xabc"}})
	})

	comments, err := client.GetCommentsByUserAddress(t.Context(), "0xabc")
	if err != nil {
		t.Fatalf("GetCommentsByUserAddress: %v", err)
	}
	if len(comments) != 1 || comments[0].UserAddress != "0xabc" {
		t.Errorf("comments = %+v", comments)
	}
}

func TestClient_GetCommentsByUserAddressPageFilters(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "10" || q.Get("offset") != "4" {
			t.Errorf("pagination = limit %q offset %q", q.Get("limit"), q.Get("offset"))
		}
		if q.Get("ascending") != "false" || q.Get("order") != "created_at" {
			t.Errorf("filters = ascending %q order %q", q.Get("ascending"), q.Get("order"))
		}
		writeJSON(w, []Comment{{ID: "c-1", UserAddress: "0xabc"}})
	})

	comments, err := client.GetCommentsByUserAddressPage(
		t.Context(),
		"0xabc",
		CommentsByUserAddressParams{
			Ascending: boolPtr(false),
			Order:     "created_at",
			Limit:     10,
			Offset:    4,
		},
	)
	if err != nil {
		t.Fatalf("GetCommentsByUserAddressPage: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
}

func TestClient_IterCommentsByUserAddressClampsOfficialLimit(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("call %d limit = %q, want 100", call, got)
		}
		switch call {
		case 1:
			comments := make([]Comment, 100)
			for i := range comments {
				comments[i] = Comment{ID: fmt.Sprintf("comment-%d", i)}
			}
			writeJSON(w, comments)
		case 2:
			if got := r.URL.Query().Get("offset"); got != "100" {
				t.Errorf("second offset = %q, want 100", got)
			}
			writeJSON(w, []Comment{{ID: "comment-100"}})
		default:
			t.Errorf("iterator made unexpected call %d", call)
		}
	})

	var ids []string
	for comment, err := range client.IterCommentsByUserAddress(
		t.Context(),
		"0xabc",
		CommentsByUserAddressParams{Limit: 150},
	) {
		if err != nil {
			t.Fatalf("IterCommentsByUserAddress: %v", err)
		}
		ids = append(ids, comment.ID)
	}
	if len(ids) != 101 || ids[100] != "comment-100" {
		t.Fatalf("collected %d comments, last %q", len(ids), ids[len(ids)-1])
	}
}

func TestClient_IterComments(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			items := make([]Comment, 2)
			for i := range items {
				items[i] = Comment{ID: string(rune('a' + i))}
			}
			writeJSON(w, items)
		} else {
			writeJSON(w, []Comment{{ID: "c"}})
		}
	})

	var ids []string
	for c, err := range client.IterComments(t.Context(), CommentFilterParams{ParentEntityType: ParentEntityTypeEvent, ParentEntityID: "123", Order: "reactionCount", Limit: 2}) {
		if err != nil {
			t.Fatalf("IterComments: %v", err)
		}
		ids = append(ids, c.ID)
	}
	if len(ids) != 3 {
		t.Errorf("ids = %v", ids)
	}
}
