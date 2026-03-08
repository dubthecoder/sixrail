// api/internal/metrolinx/client_test.go
package metrolinx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dubthecoder/railsix/api/internal/metrolinx"
)

func TestClient_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "test-key" {
			t.Errorf("expected key=test-key, got %s", r.URL.Query().Get("key"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"trains":[{"id":"1"}]}`))
	}))
	defer server.Close()

	client := metrolinx.NewClient(server.URL, "test-key")
	data, err := client.Fetch(context.Background(), "/ServiceataGlance/Trains/All")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"trains":[{"id":"1"}]}` {
		t.Fatalf("unexpected response: %s", string(data))
	}
}

func TestClient_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := metrolinx.NewClient(server.URL, "test-key")
	_, err := client.Fetch(context.Background(), "/ServiceataGlance/Trains/All")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
