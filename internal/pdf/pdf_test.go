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

func TestPDFLinkURIAbsolute(t *testing.T) {
	src := []byte(`[Go](https://go.dev) and <https://example.com/a> and [rel](./local.md) and [www](www.example.org/x)
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
	if !strings.Contains(out, "/Subtype /Link") {
		t.Fatal("missing Link annot")
	}
	if !strings.Contains(out, "/URI (https://go.dev)") {
		t.Fatalf("missing absolute URI for markdown link: %s", out)
	}
	if !strings.Contains(out, "/URI (https://example.com/a)") {
		t.Fatal("missing autolink URI")
	}
	if !strings.Contains(out, "/URI (https://www.example.org/x)") {
		t.Fatal("missing www → https URI")
	}
	if strings.Contains(out, "/URI (./local.md)") || strings.Contains(out, "local.md)") {
		t.Fatal("relative link must not get a URI annot")
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

func TestTableWrapsHeader(t *testing.T) {
	src := []byte(`| Club Log Most Wanted Rank | Maximum Grant Amount |
|---|---|
| #1 | $1,000 |
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
	for _, want := range []string{"Club", "Log", "Most", "Wanted", "Rank", "Maximum", "Grant", "Amount", "#1", "$1,000"} {
		if !strings.Contains(out, want) {
			t.Errorf("table content missing %q", want)
		}
	}
	if !strings.Contains(out, "0.90 0.90 0.90 rg") {
		t.Error("missing header row fill")
	}
}

func TestPageNumbers(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Title\n\n")
	for i := 0; i < 80; i++ {
		b.WriteString("Paragraph enough to force a second page of content in the PDF.\n\n")
	}
	doc, err := parser.Parse([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := pdf.RenderTo(doc, &buf, pdf.Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "(1) Tj") {
		t.Fatal("missing page number 1")
	}
	if !strings.Contains(out, "(2) Tj") {
		t.Fatal("missing page number 2")
	}
}
