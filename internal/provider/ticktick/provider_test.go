package ticktick

import "testing"

func TestUsesOpenAPI(t *testing.T) {
	t.Run("ticktick_uses_openapi", func(t *testing.T) {
		p := &Provider{name: "ticktick"}
		if !p.usesOpenAPI() {
			t.Fatal("expected ticktick to use openapi")
		}
	})

	t.Run("dida_uses_openapi", func(t *testing.T) {
		p := &Provider{name: "dida"}
		if !p.usesOpenAPI() {
			t.Fatal("expected dida to use openapi")
		}
	})
}

func TestIsOpenInboxProjectID(t *testing.T) {
	if !isOpenInboxProjectID("inbox") {
		t.Fatal("expected inbox alias to be recognized")
	}
	if isOpenInboxProjectID("inbox131122105") {
		t.Fatal("did not expect concrete inbox project id to be treated as alias")
	}
}
