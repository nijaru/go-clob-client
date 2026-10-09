package gamma

import (
	"net/http"
	"testing"
)

func TestClient_GetPublicProfile(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-profile" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("address") != "0xabc" {
			t.Errorf("address = %q", r.URL.Query().Get("address"))
		}
		writeJSON(w, PublicProfile{Address: "0xabc", Name: "Test User"})
	})

	profile, err := client.GetPublicProfile(t.Context(), "0xabc")
	if err != nil {
		t.Fatalf("GetPublicProfile: %v", err)
	}
	if profile.Name != "Test User" {
		t.Errorf("name = %s", profile.Name)
	}
}
