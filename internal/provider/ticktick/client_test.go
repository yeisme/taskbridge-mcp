package ticktick

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenProjectDataUsesInboxAliasEndpoint(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[],"columns":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL)
	client.openBaseURL = server.URL
	client.SetToken("tp_test")

	if _, err := client.OpenProjectData(context.Background(), openInboxProjectID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestedPath != "/project/inbox/data" {
		t.Fatalf("unexpected path: %s", requestedPath)
	}
}
