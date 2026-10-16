package docx

import (
	"bytes"
	"testing"
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
