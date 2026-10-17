package theme_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bbathe/stencil/internal/theme"
)

func TestDefault(t *testing.T) {
	th := theme.Default()
	if err := th.Validate(); err != nil {
		t.Fatal(err)
	}
	if th.Font != "Arial" || th.Mono != "Courier New" {
		t.Fatalf("fonts: %q / %q", th.Font, th.Mono)
	}
	if th.MarginTwips() != 1440 {
		t.Fatalf("margin twips: %d", th.MarginTwips())
	}
	if th.ContentWidthTwips() != 9360 {
		t.Fatalf("content width: %d", th.ContentWidthTwips())
	}
	if th.BodyHalfPoints() != 24 || th.CodeHalfPoints() != 20 {
		t.Fatalf("half-points body=%d code=%d", th.BodyHalfPoints(), th.CodeHalfPoints())
	}
}

func TestLoadPartial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.json")
	if err := os.WriteFile(path, []byte(`{"margin_in":0.75,"font":"Calibri","body_pt":11}`), 0o600); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if th.MarginIn != 0.75 || th.Font != "Calibri" || th.BodyPt != 11 {
		t.Fatalf("got %+v", th)
	}
	if th.Mono != "Courier New" || th.CodePt != 10 {
		t.Fatalf("defaults not preserved: %+v", th)
	}
	if len(th.HeadingPt) != 6 || th.HeadingPt[0] != 24 {
		t.Fatalf("heading_pt: %v", th.HeadingPt)
	}
}

func TestLoadEmptyPath(t *testing.T) {
	th, err := theme.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if th.Font != "Arial" {
		t.Fatal(th.Font)
	}
}

func TestValidateRejects(t *testing.T) {
	th := theme.Default()
	th.MarginIn = 3
	if err := th.Validate(); err == nil {
		t.Fatal("expected margin error")
	}
}

func TestNormalizePDFFonts(t *testing.T) {
	b, m := theme.NormalizePDFFonts("Aptos", "Consolas")
	if b != "Helvetica" || m != "Courier" {
		t.Fatalf("got %s / %s", b, m)
	}
}
