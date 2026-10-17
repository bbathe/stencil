package pdf_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/pdf"
)

func TestRenderBasicPDF(t *testing.T) {
	src := []byte(`# Title

Hello **bold** and *italic*.

- item one
- item two

1. first
2. second

` + "```" + `
code line
` + "```" + `

| A | B |
|---|---|
| 1 | 2 |

> a quote

---

A footnote[^1].

[^1]: Footnote body.
`)
	doc, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := pdf.RenderTo(doc, &buf, pdf.Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "%PDF-1.4") {
		t.Fatalf("missing PDF header: %q", out[:min(20, len(out))])
	}
	if !strings.Contains(out, "%%EOF") {
		t.Fatal("missing EOF")
	}
	if !strings.Contains(out, "/Helvetica") || !strings.Contains(out, "/Courier") {
		t.Fatal("missing built-in fonts")
	}
	for _, want := range []string{"Title", "Hello", "bold", "item", "one", "code line", "quote", "Notes"} {
		if !strings.Contains(out, want) {
			t.Errorf("content missing %q", want)
		}
	}
}

func TestRenderFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.pdf")
	doc, err := parser.Parse([]byte("# Hi\n\npara\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := pdf.Render(doc, out, pdf.Options{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 200 {
		t.Fatalf("pdf too small: %d", len(b))
	}
}
