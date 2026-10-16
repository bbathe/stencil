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
