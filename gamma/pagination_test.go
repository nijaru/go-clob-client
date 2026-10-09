package gamma

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestKeysetResourceEnvelopes(t *testing.T) {
	// Stable contracts: TS 087f9443 actions/{markets,events,comments}.ts and
	// bindings/gamma/{market,event,comment}.ts; Python ed8d04ca gamma specs.
	for _, resource := range []string{"markets", "events", "comments"} {
		t.Run(resource, func(t *testing.T) {
			calls := 0
			_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/"+resource+"/keyset" {
					t.Errorf("path: %s", r.URL.Path)
				}
				q := r.URL.Query()
				if q.Has("offset") || q.Get("limit") != "2" {
					t.Errorf("pagination query: %v", q)
				}
				if resource == "markets" && q.Get("order") != "volumeNum" {
					t.Errorf("market order: %v", q)
				}
				if resource == "events" &&
					(q.Get("order") != "volume" || q.Get("closed") != "false") {
					t.Errorf("event defaults/order: %v", q)
				}
				if resource == "comments" &&
					(q.Get("order") != "createdAt" || q.Get("ascending") != "false" || q.Get("parent_entity_id") != "123") {
					t.Errorf("comment defaults: %v", q)
				}
				if calls == 1 {
					if q.Has("after_cursor") {
						t.Errorf("initial cursor: %v", q)
					}
					fmt.Fprintf(
						w,
						`{"%s":[{"id":"1"}],"next_cursor":"server/opaque+token="}`,
						resource,
					)
				} else {
					if q.Get("after_cursor") != "server/opaque+token=" {
						t.Errorf("cursor: %v", q)
					}
					fmt.Fprintf(w, `{"%s":[{"id":"2"}],"next_cursor":null}`, resource)
				}
			})
			var ids []string
			switch resource {
			case "markets":
				for m, err := range client.IterMarketsKeyset(t.Context(), MarketFilterParams{Limit: 2, Order: "volume"}, "") {
					if err != nil {
						t.Fatal(err)
					}
					ids = append(ids, m.ID)
				}
			case "events":
				for e, err := range client.IterEventsKeyset(t.Context(), EventFilterParams{Limit: 2, Order: "volume"}, "") {
					if err != nil {
						t.Fatal(err)
					}
					ids = append(ids, string(e.ID))
				}
			case "comments":
				for c, err := range client.IterComments(t.Context(), CommentFilterParams{Limit: 2, ParentEntityID: "123", ParentEntityType: ParentEntityTypeEvent, Ascending: ptr(true)}) {
					if err != nil {
						t.Fatal(err)
					}
					ids = append(ids, c.ID)
				}
			}
			if calls != 2 || !slices.Equal(ids, []string{"1", "2"}) {
				t.Fatalf("calls=%d IDs=%v", calls, ids)
			}
		})
	}
}

func TestCursorQueryBinding(t *testing.T) {
	calls := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Has("after_cursor") {
			w.Write([]byte(`{"events":[],"next_cursor":null}`))
			return
		}
		w.Write([]byte(`{"events":[],"next_cursor":"token"}`))
	})
	p := EventFilterParams{TagIDs: []string{"1", "2"}, Order: "volume", Ascending: ptr(false)}
	page, err := client.GetEventsPage(t.Context(), p, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*EventFilterParams){
		func(p *EventFilterParams) { p.TagIDs = []string{"3"} },
		func(p *EventFilterParams) { p.Ascending = ptr(true) },
		func(p *EventFilterParams) { p.Closed = ptr(true) },
		func(p *EventFilterParams) { p.Locale = "de" },
		func(p *EventFilterParams) { p.Order = "liquidity" },
	} {
		changed := p
		mutate(&changed)
		if _, err := client.GetEventsPage(t.Context(), changed, page.NextCursor); !errors.Is(
			err,
			ErrInvalidCursor,
		) {
			t.Fatalf("query drift error: %v", err)
		}
	}
	if _, err := client.GetMarketsPage(t.Context(), MarketFilterParams{}, page.NextCursor); !errors.Is(
		err,
		ErrInvalidCursor,
	) {
		t.Fatalf("cross-resource cursor: %v", err)
	}
	if _, err := client.GetEventsPage(t.Context(), p, Cursor("damaged")); !errors.Is(
		err,
		ErrInvalidCursor,
	) {
		t.Fatalf("invalid cursor: %v", err)
	}
	other := New(Config{Host: client.host + "/other"})
	if _, err := other.GetEventsPage(t.Context(), p, page.NextCursor); !errors.Is(
		err,
		ErrInvalidCursor,
	) {
		t.Fatalf("cross-host cursor: %v", err)
	}
	if calls != 1 {
		t.Fatalf("drift issued %d HTTP requests", calls)
	}
	// Page size can change, and an explicit default is the same effective query.
	p.Limit = 3
	p.Closed = ptr(false)
	if next, err := client.GetEventsPage(t.Context(), p, page.NextCursor); err != nil ||
		next.HasMore {
		t.Fatalf("equivalent continuation: %+v %v", next, err)
	}
}

func TestMalformedKeysetEnvelopes(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `{}`, `{"events":null}`, `{"events":{}}`,
		`{"events":[],"next_cursor":42}`, `{"events":[],"next_cursor":""}`,
	} {
		t.Run(body, func(t *testing.T) {
			_, client := newTestServer(
				t,
				func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) },
			)
			if _, err := client.GetEventsPage(t.Context(), EventFilterParams{}, ""); err == nil {
				t.Fatalf("accepted malformed envelope: %s", body)
			}
		})
	}
}

func TestCommentKeysetDirectionAndFilters(t *testing.T) {
	for _, tc := range []struct {
		order              string
		ascending          *bool
		positions, holders bool
		path, direction    string
	}{
		{"", ptr(true), false, false, "/comments/keyset", "false"},
		{"id", nil, false, false, "/comments/keyset", "true"},
		{"createdAt", ptr(false), false, false, "/comments/keyset", "false"},
		{"reactionCount", nil, false, false, "/comments", ""},
		{"id", nil, true, false, "/comments", ""},
		{"id", nil, false, true, "/comments", ""},
	} {
		t.Run(fmt.Sprintf("%s-%t-%t", tc.order, tc.positions, tc.holders), func(t *testing.T) {
			_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.URL.Query().Get("ascending") != tc.direction {
					t.Errorf("wire: %s?%s", r.URL.Path, r.URL.RawQuery)
				}
				if tc.path == "/comments/keyset" {
					w.Write([]byte(`{"comments":[]}`))
				} else {
					w.Write([]byte(`[]`))
				}
			})
			_, err := client.GetCommentsPage(
				t.Context(),
				CommentFilterParams{
					ParentEntityID:   "123",
					ParentEntityType: ParentEntityTypeSeries,
					Order:            tc.order,
					Ascending:        tc.ascending,
					GetPositions:     ptr(tc.positions),
					HoldersOnly:      ptr(tc.holders),
				},
				"",
			)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	// Cursors are tied to parent, direction, sorting and pagination strategy.
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"comments":[],"next_cursor":"deep300"}`))
	})
	p := CommentFilterParams{
		ParentEntityID:   "123",
		ParentEntityType: ParentEntityTypeEvent,
		Order:            "id",
	}
	page, err := client.GetCommentsPage(t.Context(), p, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*CommentFilterParams){func(p *CommentFilterParams) { p.ParentEntityID = "456" }, func(p *CommentFilterParams) { p.Ascending = ptr(false) }, func(p *CommentFilterParams) { p.GetPositions = ptr(true) }} {
		q := p
		mutate(&q)
		if _, err := client.GetCommentsPage(t.Context(), q, page.NextCursor); !errors.Is(
			err,
			ErrInvalidCursor,
		) {
			t.Fatalf("comment cursor drift: %v", err)
		}
	}
}

func TestCommentOffsetCountsRootsNotReplies(t *testing.T) {
	calls := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Write(
				[]byte(
					`[{"id":"1"},{"id":"2"},{"id":"3","parentCommentID":"1"},{"id":"4","parentCommentID":"1"},{"id":"5","parentCommentID":"2"}]`,
				),
			)
			return
		}
		if r.URL.Query().Get("offset") != "2" {
			t.Errorf("root offset: %v", r.URL.Query())
		}
		w.Write(
			[]byte(
				`[{"id":"6"},{"id":"7","parentCommentID":"6"},{"id":"8","parentCommentID":"6"}]`,
			),
		)
	})
	items, err := client.ListComments(
		t.Context(),
		CommentFilterParams{
			ParentEntityID:   "123",
			ParentEntityType: ParentEntityTypeEvent,
			GetPositions:     ptr(true),
			Limit:            2,
		},
	)
	if err != nil || len(items) != 8 || calls != 2 {
		t.Fatalf("items=%d calls=%d err=%v", len(items), calls, err)
	}
}

func TestCommentListingBoundaries(t *testing.T) {
	// Both stable SDKs cap comment offsets at 200; user-address rows count
	// individually, whereas parent-entity pages count only roots.
	rows := []string{}
	for i := 0; i < 100; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":"%d"}`, i))
	}
	for _, user := range []bool{false, true} {
		t.Run(fmt.Sprintf("user=%t", user), func(t *testing.T) {
			calls := 0
			_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("offset") != "200" {
					t.Errorf("offset: %v", r.URL.Query())
				}
				w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
			})
			p := CommentFilterParams{
				ParentEntityID:   "123",
				ParentEntityType: ParentEntityTypeEvent,
				HoldersOnly:      ptr(true),
				Limit:            100,
				Offset:           200,
			}
			u := CommentsByUserAddressParams{Limit: 100, Offset: 200}
			var page *Page[Comment]
			var err error
			if user {
				page, err = client.GetUserCommentsPage(t.Context(), "wallet", u, "")
			} else {
				page, err = client.GetCommentsPage(t.Context(), p, "")
			}
			if err != nil || !page.HasMore || !page.LimitReached || page.NextCursor == "" {
				t.Fatalf("page=%+v err=%v", page, err)
			}
			if user {
				_, err = client.GetUserCommentsPage(t.Context(), "wallet", u, page.NextCursor)
			} else {
				_, err = client.GetCommentsPage(t.Context(), p, page.NextCursor)
			}
			if !errors.Is(err, ErrPaginationLimit) || calls != 1 {
				t.Fatalf("boundary follow: calls=%d err=%v", calls, err)
			}
			var items []Comment
			if user {
				items, err = client.ListCommentsByUserAddress(t.Context(), "wallet", u)
			} else {
				items, err = client.ListComments(t.Context(), p)
			}
			if !errors.Is(err, ErrPaginationLimit) || len(items) != 100 || calls != 2 {
				t.Fatalf("partial list: items=%d calls=%d err=%v", len(items), calls, err)
			}
		})
	}
}

func TestPaginationLifecycleAndGuards(t *testing.T) {
	t.Run("early stop", func(t *testing.T) {
		calls := 0
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Write([]byte(`{"events":[{"id":"1"},{"id":"2"}],"next_cursor":"token"}`))
		})
		for _, err := range client.IterEventsKeyset(t.Context(), EventFilterParams{}, "") {
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		if calls != 1 {
			t.Fatalf("calls=%d", calls)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		_, client := newTestServer(
			t,
			func(w http.ResponseWriter, r *http.Request) { t.Error("request after cancellation") },
		)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		for _, err := range client.IterEventsKeyset(ctx, EventFilterParams{}, "") {
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		}
	})
	t.Run("cursor cycle", func(t *testing.T) {
		calls := 0
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			token := "a"
			if calls == 2 {
				token = "b"
			}
			fmt.Fprintf(w, `{"events":[],"next_cursor":"%s"}`, token)
		})
		var got error
		for _, err := range client.IterEventPages(t.Context(), EventFilterParams{}, "") {
			got = err
		}
		if !errors.Is(got, ErrPaginationStalled) || calls != 3 {
			t.Fatalf("calls=%d err=%v", calls, got)
		}
	})
	t.Run("repeated offset page", func(t *testing.T) {
		calls := 0
		_, client := newTestServer(
			t,
			func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`[{"id":"1"}]`)) },
		)
		var got error
		for _, err := range client.IterSeries(t.Context(), SeriesFilterParams{Limit: 1}) {
			got = err
		}
		if !errors.Is(got, ErrPaginationStalled) || calls != 2 {
			t.Fatalf("calls=%d err=%v", calls, got)
		}
	})
	t.Run("invalid inputs", func(t *testing.T) {
		_, client := newTestServer(
			t,
			func(w http.ResponseWriter, r *http.Request) { t.Error("request for invalid pagination") },
		)
		for _, p := range []MarketFilterParams{{Offset: -1}, {Limit: -1}, {Offset: 1}} {
			if _, err := client.GetMarketsPage(t.Context(), p, ""); err == nil {
				t.Fatalf("accepted %+v", p)
			}
		}
		if _, err := client.GetComments(t.Context(), CommentFilterParams{Offset: 201}); !errors.Is(
			err,
			ErrPaginationLimit,
		) {
			t.Fatal(err)
		}
		if _, err := client.GetCommentsByUserAddressPage(t.Context(), "user", CommentsByUserAddressParams{Offset: 201}); !errors.Is(
			err,
			ErrPaginationLimit,
		) {
			t.Fatal(err)
		}
	})
}

func TestSearchBoundary(t *testing.T) {
	calls := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") != fmt.Sprint(98+calls) ||
			r.URL.Query().Get("keep_closed_markets") != "0" {
			t.Errorf("search query: %v", r.URL.Query())
		}
		w.Write([]byte(`{"events":[],"pagination":{"hasMore":true}}`))
	})
	pages := 0
	for page, err := range client.IterSearchPages(t.Context(), SearchParams{Query: "rain", Page: 99, KeepClosedMarkets: ptr(0)}) {
		if err != nil {
			t.Fatal(err)
		}
		pages++
		if pages == 2 && (!page.HasMore || !page.LimitReached || page.NextPage != 101) {
			t.Fatalf("page: %+v", page)
		}
	}
	if pages != 2 || calls != 2 {
		t.Fatalf("pages=%d calls=%d", pages, calls)
	}
	if _, err := client.GetSearchPage(t.Context(), SearchParams{Query: "rain", Page: 101}); !errors.Is(
		err,
		ErrPaginationLimit,
	) {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("issued page 101 request")
	}
}
