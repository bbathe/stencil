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
	top, right, bottom, left := th.MarginTwips()
	if top != 1440 || right != 1440 || bottom != 1440 || left != 1440 {
		t.Fatalf("margin twips: %d %d %d %d", top, right, bottom, left)
	}
	if th.ContentWidthTwips() != 9360 {
		t.Fatalf("content width: %d", th.ContentWidthTwips())
	}
	if th.BodyHalfPoints() != 24 || th.CodeHalfPoints() != 20 {
		t.Fatalf("half-points body=%d code=%d", th.BodyHalfPoints(), th.CodeHalfPoints())
	}
}

func TestLoadScalarMargin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.json")
	if err := os.WriteFile(path, []byte(`{"margin_in":0.75,"font":"Calibri","body_pt":11}`), 0o600); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if th.MarginIn.Top != 0.75 || th.MarginIn.Left != 0.75 || th.Font != "Calibri" || th.BodyPt != 11 {
		t.Fatalf("got %+v", th)
	}
	if th.Mono != "Courier New" || th.CodePt != 10 {
		t.Fatalf("defaults not preserved: %+v", th)
	}
}

func TestLoadPerSideMargin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.json")
	body := `{"margin_in":{"top":1.0,"right":0.75,"bottom":1.0,"left":0.5}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := th.MarginIn
	if m.Top != 1.0 || m.Right != 0.75 || m.Bottom != 1.0 || m.Left != 0.5 {
		t.Fatalf("margins: %+v", m)
	}
	top, right, bottom, left := th.MarginTwips()
	if top != 1440 || right != 1080 || bottom != 1440 || left != 720 {
		t.Fatalf("twips: %d %d %d %d", top, right, bottom, left)
	}
	// 12240 - 720 - 1080 = 10440
	if th.ContentWidthTwips() != 10440 {
		t.Fatalf("content width: %d", th.ContentWidthTwips())
	}
}

func TestLoadPartialSidesKeepDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.json")
	if err := os.WriteFile(path, []byte(`{"margin_in":{"left":0.5}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if th.MarginIn.Left != 0.5 {
		t.Fatal(th.MarginIn.Left)
	}
	if th.MarginIn.Top != 1.0 || th.MarginIn.Right != 1.0 || th.MarginIn.Bottom != 1.0 {
		t.Fatalf("other sides should stay default: %+v", th.MarginIn)
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
	th.MarginIn.Top = 3
	if err := th.Validate(); err == nil {
		t.Fatal("expected margin error")
	}
}
