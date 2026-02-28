package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

func TestProviderCreateTask_MoveToParent(t *testing.T) {
	moveCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/@me/lists":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "list-1", "title": "My Tasks", "updated": time.Now().Format(time.RFC3339)},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/lists/list-1/tasks":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "task-1",
				"title":   "child",
				"status":  "needsAction",
				"updated": time.Now().Format(time.RFC3339),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/lists/list-1/tasks/task-1/move":
			if got := r.URL.Query().Get("parent"); got != "parent-raw-1" {
				t.Fatalf("expected parent=parent-raw-1, got %q", got)
			}
			moveCalls++
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "task-1",
				"title":   "child",
				"status":  "needsAction",
				"parent":  "parent-raw-1",
				"updated": time.Now().Format(time.RFC3339),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	p, err := NewProvider(Config{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	p.client = NewClient("token")
	p.client.baseURL = srv.URL

	parent := "parent-raw-1"
	_, err = p.CreateTask(context.Background(), "list-1", &model.Task{
		Title:       "child",
		Status:      model.StatusTodo,
		ParentID:    &parent,
		SourceRawID: "",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if moveCalls != 1 {
		t.Fatalf("expected 1 move call, got %d", moveCalls)
	}
}

func TestProviderUpdateTask_MoveToParent(t *testing.T) {
	moveCalls := 0
	updatedBody := ""

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/users/@me/lists":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"id": "list-1", "title": "My Tasks", "updated": time.Now().Format(time.RFC3339)},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/lists/list-1/tasks/raw-2":
			var payload map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if v, ok := payload["parent"].(string); ok {
				updatedBody = v
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "raw-2",
				"title":   "child",
				"status":  "needsAction",
				"updated": time.Now().Format(time.RFC3339),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/lists/list-1/tasks/raw-2/move":
			if got := r.URL.Query().Get("parent"); got != "parent-raw-2" {
				t.Fatalf("expected parent=parent-raw-2, got %q", got)
			}
			moveCalls++
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "raw-2",
				"title":   "child",
				"status":  "needsAction",
				"parent":  "parent-raw-2",
				"updated": time.Now().Format(time.RFC3339),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	p, err := NewProvider(Config{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	p.client = NewClient("token")
	p.client.baseURL = srv.URL

	parent := "parent-raw-2"
	_, err = p.UpdateTask(context.Background(), "list-1", &model.Task{
		ID:          "google-list-1-raw-2",
		SourceRawID: "raw-2",
		Title:       "child",
		Status:      model.StatusTodo,
		ParentID:    &parent,
	})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	if !strings.Contains(updatedBody, "parent-raw-2") {
		t.Fatalf("expected update payload to carry parent, got %q", updatedBody)
	}
	if moveCalls != 1 {
		t.Fatalf("expected 1 move call, got %d", moveCalls)
	}
}

