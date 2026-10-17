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
	outPDF := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(in, []byte("# Hi\n\npara `code`\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(style, []byte(`{"margin_in":0.75,"font":"Calibri","mono":"Consolas","body_pt":11}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := converter.Run(converter.Options{InputPath: in, OutputPath: outDocx, StylePath: style}); err != nil {
		t.Fatal(err)
	}
	if err := converter.Run(converter.Options{InputPath: in, OutputPath: outPDF, StylePath: style}); err != nil {
		t.Fatal(err)
	}
	docxBytes, err := os.ReadFile(outDocx) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	// styles.xml is compressed in the zip; spot-check uncompressed length + PDF header.
	if len(docxBytes) < 100 {
		t.Fatalf("docx too small")
	}
	pdfBytes, err := os.ReadFile(outPDF) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if string(pdfBytes[:5]) != "%PDF-" {
		t.Fatal("pdf header")
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
