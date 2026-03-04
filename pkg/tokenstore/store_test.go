package tokenstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeisme/taskbridge/pkg/paths"
)

func TestSaveLoadDeleteSingleFile(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "credentials", "tokens.json")

	googleToken := map[string]string{"access_token": "google-token"}
	msToken := map[string]string{"access_token": "ms-token"}

	if err := Save(tokenPath, "google", googleToken); err != nil {
		t.Fatalf("Save google failed: %v", err)
	}
	if err := Save(tokenPath, "microsoft", msToken); err != nil {
		t.Fatalf("Save microsoft failed: %v", err)
	}

	hasGoogle, err := Has(tokenPath, "google")
	if err != nil {
		t.Fatalf("Has google failed: %v", err)
	}
	if !hasGoogle {
		t.Fatal("expected google token to exist")
	}

	hasMicrosoft, err := Has(tokenPath, "microsoft")
	if err != nil {
		t.Fatalf("Has microsoft failed: %v", err)
	}
	if !hasMicrosoft {
		t.Fatal("expected microsoft token to exist")
	}

	if err := Delete(tokenPath, "google"); err != nil {
		t.Fatalf("Delete google failed: %v", err)
	}

	hasGoogle, err = Has(tokenPath, "google")
	if err != nil {
		t.Fatalf("Has google after delete failed: %v", err)
	}
	if hasGoogle {
		t.Fatal("expected google token to be removed")
	}

	hasMicrosoft, err = Has(tokenPath, "microsoft")
	if err != nil {
		t.Fatalf("Has microsoft after google delete failed: %v", err)
	}
	if !hasMicrosoft {
		t.Fatal("expected microsoft token to remain")
	}
}

func TestLoadFallbackToLegacyFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TASKBRIDGE_HOME", home)

	legacyPath := paths.GetLegacyTokenPath("google")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	legacyToken := map[string]string{"access_token": "legacy-google-token"}
	legacyData, err := json.Marshal(legacyToken)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := os.WriteFile(legacyPath, legacyData, 0600); err != nil {
		t.Fatalf("write legacy token failed: %v", err)
	}

	tokenPath := paths.GetTokenPath("google")
	hasGoogle, err := Has(tokenPath, "google")
	if err != nil {
		t.Fatalf("Has google failed: %v", err)
	}
	if !hasGoogle {
		t.Fatal("expected google token to be found from legacy file")
	}

	var loaded map[string]string
	if err := Load(tokenPath, "google", &loaded); err != nil {
		t.Fatalf("Load google failed: %v", err)
	}
	if loaded["access_token"] != "legacy-google-token" {
		t.Fatalf("unexpected legacy token: %s", loaded["access_token"])
	}
}

func TestSaveRemovesLegacyFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TASKBRIDGE_HOME", home)

	legacyPath := paths.GetLegacyTokenPath("todoist")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"api_token":"old"}`), 0600); err != nil {
		t.Fatalf("write legacy token failed: %v", err)
	}

	tokenPath := paths.GetTokenPath("todoist")
	if err := Save(tokenPath, "todoist", map[string]string{"api_token": "new"}); err != nil {
		t.Fatalf("Save todoist failed: %v", err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy token file removed, got err=%v", err)
	}

	hasTodoist, err := Has(tokenPath, "todoist")
	if err != nil {
		t.Fatalf("Has todoist failed: %v", err)
	}
	if !hasTodoist {
		t.Fatal("expected todoist token to exist in shared file")
	}
}
