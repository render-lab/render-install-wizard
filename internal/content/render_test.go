package content

import "testing"

func TestRenderNonEmpty(t *testing.T) {
	out := Render("# Title\n\nBody text.")
	if out == "" {
		t.Fatal("Render returned empty string for non-empty input")
	}
}

func TestRenderEmptyDoesNotPanic(t *testing.T) {
	_ = Render("")
}

func TestRenderPlainStrips(t *testing.T) {
	if got := RenderPlain("  # Title  \n"); got != "# Title" {
		t.Errorf("RenderPlain = %q, want %q", got, "# Title")
	}
}

func TestEmbeddedCursor(t *testing.T) {
	md, ok := Embedded("cursor")
	if !ok {
		t.Fatal("Embedded(cursor) ok = false, want true")
	}
	if md == "" {
		t.Error("Embedded(cursor) returned empty snapshot")
	}
}

func TestEmbeddedUnknown(t *testing.T) {
	if _, ok := Embedded("nope"); ok {
		t.Error("Embedded(nope) ok = true, want false")
	}
}
