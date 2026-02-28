package mcp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGetStringSlice(t *testing.T) {
	raw := map[string]json.RawMessage{
		"single": json.RawMessage(`"microsoft"`),
		"array":  json.RawMessage(`["ms","google"]`),
	}

	gotSingle := getStringSlice(raw, "single")
	if !reflect.DeepEqual(gotSingle, []string{"microsoft"}) {
		t.Fatalf("unexpected single result: %#v", gotSingle)
	}

	gotArray := getStringSlice(raw, "array")
	if !reflect.DeepEqual(gotArray, []string{"ms", "google"}) {
		t.Fatalf("unexpected array result: %#v", gotArray)
	}
}

func TestGetIntSlice(t *testing.T) {
	raw := map[string]json.RawMessage{
		"single": json.RawMessage(`2`),
		"array":  json.RawMessage(`[1,3,4]`),
	}

	gotSingle := getIntSlice(raw, "single")
	if !reflect.DeepEqual(gotSingle, []int{2}) {
		t.Fatalf("unexpected single result: %#v", gotSingle)
	}

	gotArray := getIntSlice(raw, "array")
	if !reflect.DeepEqual(gotArray, []int{1, 3, 4}) {
		t.Fatalf("unexpected array result: %#v", gotArray)
	}
}

func TestSanitizeMarkdownText(t *testing.T) {
	got := sanitizeMarkdownText("[ ] **设计免费版与付费版功能矩阵**：定义 Free/Pro/Enterprise")
	want := "设计免费版与付费版功能矩阵：定义 Free/Pro/Enterprise"
	if got != want {
		t.Fatalf("unexpected sanitize result: got=%q want=%q", got, want)
	}
}

func TestBuildMicrosoftStepLocalID(t *testing.T) {
	got := buildMicrosoftStepLocalID("parent-raw-id", "step-id")
	want := "ms-step-parent-raw-id-step-id"
	if got != want {
		t.Fatalf("unexpected ms step local id: got=%q want=%q", got, want)
	}
}

func TestParseMicrosoftStepID(t *testing.T) {
	got := parseMicrosoftStepID("ms_step:1413839e-bb07-4d2b-a32d-00baa44f793e")
	want := "1413839e-bb07-4d2b-a32d-00baa44f793e"
	if got != want {
		t.Fatalf("unexpected ms step id: got=%q want=%q", got, want)
	}
}
