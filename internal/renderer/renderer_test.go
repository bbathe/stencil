package renderer_test

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/renderer"
	"github.com/bbathe/stencil/internal/stylemap"
)

func TestGoldenFeatures(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "pic.png")
	writeTinyPNG(t, imgPath)

	md := strings.Join([]string{
		"# Title",
		"",
		"Soft",
		"break here.",
		"",
		"Hard  ",
		"break here.",
		"",
		"> quoted",
		"",
		"~~strike~~ and **bold**",
		"",
		"3. third",
		"4. fourth",
		"",
		"| Left | Center | Right |",
		"| :--- | :----: | ----: |",
		"| a | b | c |",
		"",
		"See note.[^1]",
		"",
		"![alt](pic.png)",
		"",
		"[^1]: Footnote with a [link](https://example.com).",
		"",
	}, "\n")

	doc, err := parser.Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	sm, err := stylemap.Load("")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := renderer.RenderTo(doc, &buf, renderer.Options{BaseDir: dir, StyleMap: sm}); err != nil {
		t.Fatal(err)
	}

	// Determinism: second render must match.
	var buf2 bytes.Buffer
	if err := renderer.RenderTo(doc, &buf2, renderer.Options{BaseDir: dir, StyleMap: sm}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), buf2.Bytes()) {
		t.Fatal("RenderTo not deterministic")
	}

	parts := unzip(t, buf.Bytes())
	docXML := string(parts["word/document.xml"])
	numXML := string(parts["word/numbering.xml"])
	fnXML := string(parts["word/footnotes.xml"])
	fnRels := string(parts["word/_rels/footnotes.xml.rels"])

	checks := []struct {
		name   string
		hay    string
		needle string
	}{
		{"heading style", docXML, `w:pStyle w:val="Heading1"`},
		{"blockquote style", docXML, `w:pStyle w:val="Quote"`},
		// Soft break inserts a separate space run between line fragments.
		{"soft break space", docXML, `xml:space="preserve"> </w:t>`},
		{"strikethrough", docXML, `<w:strike/>`},
		{"table center", docXML, `<w:jc w:val="center"/>`},
		{"table right", docXML, `<w:jc w:val="right"/>`},
		{"list start 3", numXML, `w:startOverride w:val="3"`},
		{"image drawing id", docXML, `wp:docPr id="1"`},
		{"footnote link", fnXML, `w:hyperlink r:id="`},
		{"footnote rels", fnRels, `TargetMode="External"`},
		{"footnote xmlns r", fnXML, `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`},
	}
	for _, c := range checks {
		if !strings.Contains(c.hay, c.needle) {
			t.Errorf("%s: missing %q", c.name, c.needle)
		}
	}

	if _, ok := parts["word/media/image1.png"]; !ok {
		t.Error("embedded image missing from package")
	}
}

func TestImageRelativeToBaseDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "assets")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTinyPNG(t, filepath.Join(sub, "logo.png"))

	md := "![logo](assets/logo.png)\n"
	doc, err := parser.Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	sm, _ := stylemap.Load("")
	out := filepath.Join(dir, "out.docx")
	if err := renderer.Render(doc, out, renderer.Options{BaseDir: dir, StyleMap: sm}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	parts := unzip(t, data)
	if _, ok := parts["word/media/image1.png"]; !ok {
		t.Fatal("image not embedded when path is relative to markdown dir")
	}
}

func TestRemoteImageFallsBack(t *testing.T) {
	md := "![x](https://example.com/a.png)\n"
	doc, err := parser.Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	sm, _ := stylemap.Load("")
	var buf bytes.Buffer
	if err := renderer.RenderTo(doc, &buf, renderer.Options{StyleMap: sm}); err != nil {
		t.Fatal(err)
	}
	parts := unzip(t, buf.Bytes())
	docXML := string(parts["word/document.xml"])
	if !strings.Contains(docXML, "[image:") {
		t.Fatalf("expected alt-text fallback, got:\n%s", docXML)
	}
}

func TestRichTextInsideLinks(t *testing.T) {
	md := "See [**bold** and `code` and ~~x~~](https://example.com/path).\n"
	doc, err := parser.Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	sm, _ := stylemap.Load("")
	var buf bytes.Buffer
	if err := renderer.RenderTo(doc, &buf, renderer.Options{StyleMap: sm}); err != nil {
		t.Fatal(err)
	}
	docXML := string(unzip(t, buf.Bytes())["word/document.xml"])
	if !strings.Contains(docXML, `<w:hyperlink r:id="`) {
		t.Fatal("missing hyperlink")
	}
	// Bold/strike/code must appear inside the hyperlink element.
	i := strings.Index(docXML, `<w:hyperlink`)
	j := strings.Index(docXML[i:], `</w:hyperlink>`)
	if i < 0 || j < 0 {
		t.Fatal("hyperlink bounds missing")
	}
	inner := docXML[i : i+j]
	for _, needle := range []string{`<w:b/>`, `<w:strike/>`, `Courier New`} {
		if !strings.Contains(inner, needle) {
			t.Errorf("hyperlink missing %q in:\n%s", needle, inner)
		}
	}
}

func TestUnhandledNodeErrors(t *testing.T) {
	// Definition lists are not part of GFM / stencil's parser, but if they
	// appear in the AST the renderer must fail instead of dropping them.
	src := []byte("Term\n: Definition\n")
	md := goldmark.New(goldmark.WithExtensions(extension.DefinitionList))
	node := md.Parser().Parse(text.NewReader(src))
	doc := &parser.Document{Node: node, Source: src}
	sm, _ := stylemap.Load("")
	var buf bytes.Buffer
	err := renderer.RenderTo(doc, &buf, renderer.Options{StyleMap: sm})
	if err == nil {
		t.Fatal("expected error for unhandled definition-list nodes")
	}
	if !strings.Contains(err.Error(), "unhandled markdown nodes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeTinyPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func unzip(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			_ = rc.Close()
			t.Fatal(err)
		}
		_ = rc.Close()
		out[zf.Name] = buf.Bytes()
	}
	return out
}
