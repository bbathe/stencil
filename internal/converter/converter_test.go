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

func TestRunPDF(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "doc.md")
	out := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(in, []byte("# Hi\n\npara\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := converter.Run(converter.Options{InputPath: in, OutputPath: out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 100 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a pdf: size=%d head=%q", len(b), string(b[:min(8, len(b))]))
	}
}
