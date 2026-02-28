package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateTask_WithParentQueryParam(t *testing.T) {
	var gotParent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotParent = r.URL.Query().Get("parent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "child-1",
			"title":  "child",
			"status": "needsAction",
			"parent": gotParent,
		})
	}))
	defer srv.Close()

	c := NewClient("test-token")
	c.baseURL = srv.URL

	task := &Task{
		Title:  "child",
		Status: "needsAction",
		Parent: "parent-123",
	}
	if _, err := c.CreateTask(context.Background(), "list-1", task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if gotParent != "parent-123" {
		t.Fatalf("expected parent query parent-123, got %q", gotParent)
	}
}

