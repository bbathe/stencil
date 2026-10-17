package docx

import (
	"bytes"
	"testing"

	"github.com/bbathe/stencil/internal/theme"
)

func TestDeterministicWrite(t *testing.T) {
	build := func() []byte {
		d := New()
		p := d.AddParagraph("Normal")
		p.AddRun("hello")
		var buf bytes.Buffer
		if err := d.Write(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	a, b := build(), build()
	if !bytes.Equal(a, b) {
		t.Fatalf("docx output not deterministic (%d vs %d bytes)", len(a), len(b))
	}
}

func TestTableHasGridWidths(t *testing.T) {
	d := New()
	tbl := d.AddTable()
	hdr := tbl.AddRow(true)
	hdr.AddCell("Normal").AddRun("A")
	hdr.AddCell("Normal").AddRun("B")
	row := tbl.AddRow(false)
	row.AddCell("Normal").AddRun("1")
	row.AddCell("Normal").AddRun("2")

	xml, err := d.documentXML()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(xml, []byte(`<w:tblGrid>`)) {
		t.Fatal("missing tblGrid")
	}
	if !bytes.Contains(xml, []byte(`w:gridCol w:w="`)) {
		t.Fatal("missing gridCol widths")
	}
	if bytes.Contains(xml, []byte(`w:tcW w:w="0"`)) {
		t.Fatal("cell width should not be zero")
	}
	if !bytes.Contains(xml, []byte(`w:tblLayout w:type="fixed"`)) {
		t.Fatal("expected fixed table layout")
	}
}

func TestEqualColumnWidths(t *testing.T) {
	got := equalColumnWidths(2, 9360)
	if len(got) != 2 || got[0]+got[1] != 9360 {
		t.Fatalf("got %v", got)
	}
	if got[0] != 4680 || got[1] != 4680 {
		t.Fatalf("uneven split: %v", got)
	}
}

func TestDefaultStylesUseNamedFonts(t *testing.T) {
	styles := stylesXML(nil)
	if bytes.Contains(styles, []byte("asciiTheme")) {
		t.Fatal("built-in styles must not use theme font references")
	}
	if !bytes.Contains(styles, []byte(`w:ascii="Arial"`)) {
		t.Fatal("expected Arial as default body font")
	}
	if !bytes.Contains(styles, []byte(`w:ascii="Courier New"`)) {
		t.Fatal("expected Courier New for Code style")
	}
}

func TestThemedStyles(t *testing.T) {
	th := theme.Default()
	th.Font = "Calibri"
	th.Mono = "Consolas"
	th.BodyPt = 11
	th.MarginIn = 0.75
	styles := stylesXML(th)
	if !bytes.Contains(styles, []byte(`w:ascii="Calibri"`)) {
		t.Fatal("expected Calibri")
	}
	if !bytes.Contains(styles, []byte(`w:ascii="Consolas"`)) {
		t.Fatal("expected Consolas")
	}
	if !bytes.Contains(styles, []byte(`w:sz w:val="22"`)) { // 11pt
		t.Fatal("expected 11pt body half-points")
	}
	d := New()
	d.Theme = th
	d.AddParagraph("Normal").AddRun("x")
	var buf bytes.Buffer
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	// Margins appear in document.xml uncompressed stream inside zip — check styles path via stylesXML only.
	_ = buf
}

func TestCorePropsNoAuthor(t *testing.T) {
	d := New()
	data, err := d.coreProps()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("dc:creator")) || bytes.Contains(data, []byte("lastModifiedBy")) {
		t.Fatalf("core props should omit author fields:\n%s", data)
	}
	if !bytes.Contains(data, []byte("dcterms:created")) || !bytes.Contains(data, []byte("dcterms:modified")) {
		t.Fatalf("core props should keep fixed dates:\n%s", data)
	}
}

func TestOrderedListStart(t *testing.T) {
	d := New()
	id := d.NewOrderedNumID(5)
	if id != 2 {
		t.Fatalf("numId: got %d", id)
	}
	data, err := d.numbering()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`w:startOverride w:val="5"`)) {
		t.Fatalf("missing start override:\n%s", data)
	}
}

func TestUniqueDrawingIDs(t *testing.T) {
	d := New()
	// RegisterImage needs a real file; use empty path failure path for IDs via AddImageItem directly.
	p := d.AddParagraph("Normal")
	p.AddImageItem("rIdIM1", "a", 100, 100, 1)
	p.AddImageItem("rIdIM2", "b", 100, 100, 2)
	xml, err := d.documentXML()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(xml, []byte(`wp:docPr id="1"`)) || !bytes.Contains(xml, []byte(`wp:docPr id="2"`)) {
		t.Fatalf("expected unique drawing ids:\n%s", xml)
	}
}
