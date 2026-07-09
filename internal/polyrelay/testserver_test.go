package polyrelay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// newJSONServer returns a test server that JSON-encodes the handler's return
// value for every request. Returning a *cannedError writes its status+body instead.
func newJSONServer(handler func(*http.Request) any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := handler(r)
		if e, ok := resp.(*cannedError); ok {
			w.WriteHeader(e.status)
			_, _ = w.Write([]byte(e.body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
