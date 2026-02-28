package todoist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListSections(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			_, _ = w.Write([]byte(`{"results":[{"id":"s1","project_id":"p1","name":"Board A"}],"next_cursor":"next"}`))
		case 2:
			_, _ = w.Write([]byte(`{"results":[{"id":"s2","project_id":"p1","name":"Board B"}],"next_cursor":""}`))
		default:
			t.Fatalf("unexpected extra request: %s", r.URL.String())
		}
	}))
	defer server.Close()

	client := NewClient("token")
	client.baseURL = server.URL

	sections, err := client.ListSections(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
}
