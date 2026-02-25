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
