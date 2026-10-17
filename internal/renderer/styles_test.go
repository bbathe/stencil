package renderer

import "testing"

func TestStyleName(t *testing.T) {
	if got := styleName("blockquote"); got != "Quote" {
		t.Fatalf("blockquote: got %q", got)
	}
	if got := styleName("heading1"); got != "Heading 1" {
		t.Fatalf("heading1: got %q", got)
	}
	if got := styleName("missing"); got != "Normal" {
		t.Fatalf("missing: got %q", got)
	}
}
