package stylemap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	sm, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if got := sm.Style("blockquote"); got != "Quote" {
		t.Fatalf("blockquote: got %q", got)
	}
	if got := sm.Style("missing"); got != "Normal" {
		t.Fatalf("missing key: got %q", got)
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "map.json")
	if err := os.WriteFile(path, []byte(`{"body":"Body Text","blockquote":"MyQuote"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sm, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := sm.Style("body"); got != "Body Text" {
		t.Fatalf("body: got %q", got)
	}
	if got := sm.Style("blockquote"); got != "MyQuote" {
		t.Fatalf("blockquote: got %q", got)
	}
	if got := sm.Style("heading1"); got != "Heading 1" {
		t.Fatalf("heading1 fallback: got %q", got)
	}
}
