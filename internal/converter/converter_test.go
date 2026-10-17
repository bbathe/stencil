package converter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bbathe/stencil/internal/converter"
)

func TestRunFileToFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	out := filepath.Join(dir, "doc.docx")
	if err := os.WriteFile(in, []byte("# Hi\n\npara\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := converter.Run(converter.Options{
		InputPath:  in,
		OutputPath: out,
	})
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() < 100 {
		t.Fatalf("docx too small: %d", fi.Size())
	}
}

func TestRunWithStyle(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	style := filepath.Join(dir, "style.json")
	outDocx := filepath.Join(dir, "doc.docx")
	if err := os.WriteFile(in, []byte("# Hi\n\npara `code`\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(style, []byte(`{"margin_in":0.75,"font":"Calibri","mono":"Consolas","body_pt":11}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := converter.Run(converter.Options{InputPath: in, OutputPath: outDocx, StylePath: style}); err != nil {
		t.Fatal(err)
	}
	docxBytes, err := os.ReadFile(outDocx) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if len(docxBytes) < 100 {
		t.Fatalf("docx too small")
	}
}
