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
		writeJSON(w, PublicProfile{Address: ptr("0xabc"), Name: ptr("Test User")})
	})

	profile, err := client.GetPublicProfile(t.Context(), "0xabc")
	if err != nil {
		t.Fatalf("GetPublicProfile: %v", err)
	}
	if profile.Name == nil || *profile.Name != "Test User" {
		t.Errorf("name = %v", profile.Name)
	}
}
