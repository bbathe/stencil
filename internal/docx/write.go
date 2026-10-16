package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Serialisation ─────────────────────────────────────────────────────────────

// SaveToFile writes the complete .docx to path.
func (d *Document) SaveToFile(path string) error {
	f, err := os.Create(path) //nolint:gosec // path is caller-supplied output flag
	if err != nil {
		return fmt.Errorf("creating %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return d.Write(f)
}

// Write serialises the .docx to w.
func (d *Document) Write(w io.Writer) (retErr error) {
	zw := zip.NewWriter(w)
	defer func() {
		if cerr := zw.Close(); cerr != nil && retErr == nil {
			retErr = cerr
		}
	}()

	type namedPart struct {
		name string
		fn   func() ([]byte, error)
	}

	parts := []namedPart{
		{"[Content_Types].xml", d.contentTypes},
		{"_rels/.rels", d.rootRels},
		{"docProps/core.xml", d.coreProps},
		{"word/_rels/document.xml.rels", d.documentRels},
		{"word/settings.xml", d.settings},
		{"word/numbering.xml", d.numbering},
		{"word/document.xml", d.documentXML},
	}

	for _, p := range parts {
		data, err := p.fn()
		if err != nil {
			return fmt.Errorf("generating %s: %w", p.name, err)
		}
		if err := writeEntry(zw, p.name, data); err != nil {
			return err
		}
	}

	// Styles — use template's or built-in defaults
	styles := d.styles
	if styles == nil {
		styles = defaultStyles()
	}
	if err := writeEntry(zw, "word/styles.xml", styles); err != nil {
		return err
	}

	// Footnotes — only if present
	if len(d.footnotes) > 0 {
		data, err := d.footnotesXML()
		if err != nil {
			return fmt.Errorf("generating footnotes.xml: %w", err)
		}
		if err := writeEntry(zw, "word/footnotes.xml", data); err != nil {
			return err
		}
		rels, err := d.footnotesRels()
		if err != nil {
			return fmt.Errorf("generating footnotes.xml.rels: %w", err)
		}
		if err := writeEntry(zw, "word/_rels/footnotes.xml.rels", rels); err != nil {
			return err
		}
	}

	// Images
	for _, img := range d.images {
		if err := writeEntry(zw, img.partName, img.data); err != nil {
			return err
		}
	}

	return nil
}

// ── XML part generators ───────────────────────────────────────────────────────

func (d *Document) contentTypes() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml"  ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/word/document.xml"  ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>`)
	b.WriteString(`<Override PartName="/word/styles.xml"    ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>`)
	b.WriteString(`<Override PartName="/word/settings.xml"  ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>`)
	b.WriteString(`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>`)
	b.WriteString(`<Override PartName="/docProps/core.xml"  ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	if len(d.footnotes) > 0 {
		b.WriteString(`<Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/>`)
	}
	// Image defaults (by extension, deduped)
	seenExt := map[string]bool{}
	for _, img := range d.images {
		if !seenExt[img.ext] {
			fmt.Fprintf(&b, `<Default Extension="%s" ContentType="%s"/>`, img.ext, img.mimeType)
			seenExt[img.ext] = true
		}
	}
	b.WriteString(`</Types>`)
	return b.Bytes(), nil
}

func (d *Document) rootRels() ([]byte, error) {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
</Relationships>`), nil
}

func (d *Document) coreProps() ([]byte, error) {
	// Creator / lastModifiedBy omitted (optional in OPC). Fixed timestamps keep
	// package bytes deterministic without attributing authorship.
	ts := fixedZipTime.Format(time.RFC3339)
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties
    xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
    xmlns:dcterms="http://purl.org/dc/terms/"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, ts, ts), nil
}

func (d *Document) documentRels() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	b.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"    Target="styles.xml"/>`)
	b.WriteString(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"   Target="settings.xml"/>`)
	b.WriteString(`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"  Target="numbering.xml"/>`)
	if len(d.footnotes) > 0 {
		b.WriteString(`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>`)
	}
	for _, hl := range d.hyperlinks {
		fmt.Fprintf(&b,
			`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`,
			hl.rId, xmlAttr(hl.url))
	}
	for _, img := range d.images {
		fmt.Fprintf(&b,
			`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`,
			img.rId, filepath.Base(img.partName))
	}
	b.WriteString(`</Relationships>`)
	return b.Bytes(), nil
}

func (d *Document) settings() ([]byte, error) {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:defaultTabStop w:val="720"/>
  <w:compat>
    <w:compatSetting w:name="compatibilityMode"
      w:uri="http://schemas.microsoft.com/office/word"
      w:val="15"/>
  </w:compat>
</w:settings>`), nil
}

func (d *Document) numbering() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + "\n")

	// ── Abstract 0: bullets (9 levels) ───────────────────────────────────────
	bullets := []string{`&#x2022;`, `o`, `&#x25AA;`, `&#x2022;`, `o`, `&#x25AA;`, `&#x2022;`, `o`, `&#x25AA;`}
	b.WriteString(`<w:abstractNum w:abstractNumId="0"><w:multiLevelType w:val="hybridMultilevel"/>`)
	for i := range 9 {
		left := (i + 1) * 720
		fmt.Fprintf(&b,
			`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="bullet"/>`+
				`<w:lvlText w:val="%s"/><w:lvlJc w:val="left"/>`+
				`<w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`,
			i, bullets[i], left)
	}
	b.WriteString(`</w:abstractNum>` + "\n")

	// ── Abstract 1: decimal ordered (9 levels) ────────────────────────────────
	b.WriteString(`<w:abstractNum w:abstractNumId="1"><w:multiLevelType w:val="multilevel"/>`)
	for i := range 9 {
		left := (i + 1) * 720
		fmt.Fprintf(&b,
			`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="decimal"/>`+
				`<w:lvlText w:val="%%%d."/><w:lvlJc w:val="left"/>`+
				`<w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`,
			i, i+1, left)
	}
	b.WriteString(`</w:abstractNum>` + "\n")

	// ── Num 1: bullets (shared) ───────────────────────────────────────────────
	b.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` + "\n")

	// ── Nums 2..N: one per ordered-list instance (restart / custom start) ─────
	for i, start := range d.orderedListStarts {
		numId := i + 2
		fmt.Fprintf(&b,
			`<w:num w:numId="%d"><w:abstractNumId w:val="1"/>`+
				`<w:lvlOverride w:ilvl="0"><w:startOverride w:val="%d"/></w:lvlOverride>`+
				`</w:num>`+"\n", numId, start)
	}

	b.WriteString(`</w:numbering>`)
	return b.Bytes(), nil
}

func (d *Document) documentXML() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<w:document` +
		` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` + "\n")
	b.WriteString(`<w:body>` + "\n")

	for _, el := range d.body {
		switch v := el.(type) {
		case *Para:
			writePara(&b, v)
		case *Table:
			writeTable(&b, v)
		}
	}

	// Section properties with proper letter-page margins (1 inch all around)
	b.WriteString(`<w:sectPr>` +
		`<w:pgSz w:w="12240" w:h="15840"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/>` +
		`</w:sectPr>` + "\n")
	b.WriteString(`</w:body></w:document>`)
	return b.Bytes(), nil
}

func (d *Document) footnotesXML() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<w:footnotes` +
		` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">`)

	// Required separator footnotes
	b.WriteString(`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>`)
	b.WriteString(`<w:footnote w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:footnote>`)

	for _, fn := range d.footnotes {
		fmt.Fprintf(&b, `<w:footnote w:id="%d"><w:p>`, fn.id)
		b.WriteString(`<w:pPr><w:pStyle w:val="FootnoteText"/></w:pPr>`)
		b.WriteString(`<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:footnoteRef/></w:r>`)
		b.WriteString(`<w:r><w:t xml:space="preserve"> </w:t></w:r>`)
		// Render the Para items (text runs, hyperlinks, images, etc.)
		if fn.para != nil {
			for _, item := range fn.para.items {
				switch v := item.(type) {
				case textRun:
					writeRun(&b, v, false)
				case hyperlinkItem:
					fmt.Fprintf(&b, `<w:hyperlink r:id="%s">`, v.rId)
					for _, r := range v.runs {
						writeRun(&b, r, false)
					}
					b.WriteString(`</w:hyperlink>`)
				case imageItem:
					writeImageRun(&b, v)
				}
			}
		}
		b.WriteString(`</w:p></w:footnote>`)
	}

	b.WriteString(`</w:footnotes>`)
	return b.Bytes(), nil
}

// footnotesRels emits relationships for hyperlinks and images referenced from
// footnotes.xml. Word resolves r:id against this part, not document.xml.rels.
func (d *Document) footnotesRels() ([]byte, error) {
	usedHL := map[string]bool{}
	usedIM := map[string]bool{}
	for _, fn := range d.footnotes {
		if fn.para == nil {
			continue
		}
		for _, item := range fn.para.items {
			switch v := item.(type) {
			case hyperlinkItem:
				usedHL[v.rId] = true
			case imageItem:
				usedIM[v.rId] = true
			}
		}
	}

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, hl := range d.hyperlinks {
		if !usedHL[hl.rId] {
			continue
		}
		fmt.Fprintf(&b,
			`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`,
			hl.rId, xmlAttr(hl.url))
	}
	for _, img := range d.images {
		if !usedIM[img.rId] {
			continue
		}
		fmt.Fprintf(&b,
			`<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`,
			img.rId, filepath.Base(img.partName))
	}
	b.WriteString(`</Relationships>`)
	return b.Bytes(), nil
}

// ── XML element writers ───────────────────────────────────────────────────────

func writePara(b *bytes.Buffer, p *Para) {
	b.WriteString(`<w:p>`)

	// Paragraph properties
	needPPr := p.style != "" || p.numID > 0 || p.blockquoteDepth > 0 || p.hrule || p.align != AlignDefault
	if needPPr {
		b.WriteString(`<w:pPr>`)
		if p.style != "" {
			b.WriteString(`<w:pStyle w:val="`)
			_ = xml.EscapeText(b, []byte(styleID(p.style)))
			b.WriteString(`"/>`)
		}
		if p.numID > 0 {
			fmt.Fprintf(b, `<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, p.numILvl, p.numID)
		}
		if p.blockquoteDepth > 0 {
			b.WriteString(`<w:pBdr><w:left w:val="single" w:sz="24" w:space="4" w:color="CCCCCC"/></w:pBdr>`)
			// Additional indentation per nesting level (720 twips = 0.5 in per level)
			indent := p.blockquoteDepth * 720
			fmt.Fprintf(b, `<w:ind w:left="%d" w:right="%d"/>`, indent, indent)
		}
		if p.hrule {
			b.WriteString(`<w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="AAAAAA"/></w:pBdr>`)
		}
		switch p.align {
		case AlignLeft:
			b.WriteString(`<w:jc w:val="left"/>`)
		case AlignCenter:
			b.WriteString(`<w:jc w:val="center"/>`)
		case AlignRight:
			b.WriteString(`<w:jc w:val="right"/>`)
		}
		b.WriteString(`</w:pPr>`)
	}

	for _, item := range p.items {
		switch v := item.(type) {
		case textRun:
			writeRun(b, v, p.forceBold)
		case hyperlinkItem:
			fmt.Fprintf(b, `<w:hyperlink r:id="%s">`, v.rId)
			for _, r := range v.runs {
				writeRun(b, r, p.forceBold)
			}
			b.WriteString(`</w:hyperlink>`)
		case imageItem:
			writeImageRun(b, v)
		case footnoteRefItem:
			fmt.Fprintf(b, `<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:footnoteReference w:id="%d"/></w:r>`, v.id)
		}
	}

	b.WriteString(`</w:p>` + "\n")
}

func writeRun(b *bytes.Buffer, r textRun, forceBold bool) {
	b.WriteString(`<w:r>`)

	bold := r.bold || forceBold
	hasProps := bold || r.italic || r.code || r.strike || r.underline || r.color != ""
	if hasProps {
		b.WriteString(`<w:rPr>`)
		if bold {
			b.WriteString(`<w:b/>`)
		}
		if r.italic {
			b.WriteString(`<w:i/>`)
		}
		if r.strike {
			b.WriteString(`<w:strike/>`)
		}
		if r.underline {
			b.WriteString(`<w:u w:val="single"/>`)
		}
		if r.color != "" {
			fmt.Fprintf(b, `<w:color w:val="%s"/>`, r.color)
		}
		if r.code {
			b.WriteString(`<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:cs="Courier New"/>`)
			b.WriteString(`<w:sz w:val="20"/>`)
		}
		b.WriteString(`</w:rPr>`)
	}

	if r.text != "" {
		b.WriteString(`<w:t xml:space="preserve">`)
		_ = xml.EscapeText(b, []byte(r.text))
		b.WriteString(`</w:t>`)
	}
	if r.hardBreak {
		b.WriteString(`<w:br/>`)
	}

	b.WriteString(`</w:r>`)
}

func writeImageRun(b *bytes.Buffer, img imageItem) {
	var alt bytes.Buffer
	_ = xml.EscapeText(&alt, []byte(img.altText))
	altS := alt.String()
	id := max(img.drawingID, 1)

	fmt.Fprintf(b,
		`<w:r><w:rPr/><w:drawing>`+
			`<wp:inline>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:docPr id="%d" name="Image%d" descr="%s"/>`+
			`<a:graphic>`+
			`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic>`+
			`<pic:nvPicPr><pic:cNvPr id="%d" name="Image%d" descr="%s"/><pic:cNvPicPr/></pic:nvPicPr>`+
			`<pic:blipFill>`+
			`<a:blip r:embed="%s"/>`+
			`<a:stretch><a:fillRect/></a:stretch>`+
			`</pic:blipFill>`+
			`<pic:spPr>`+
			`<a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+
			`</pic:spPr>`+
			`</pic:pic></a:graphicData></a:graphic>`+
			`</wp:inline></w:drawing></w:r>`,
		img.widthEMU, img.heightEMU, id, id, altS, id, id, altS, img.rId, img.widthEMU, img.heightEMU)
}

// usablePageWidthTwips is the content width for letter page with 1" margins
// (12240 - 2*1440), matching documentXML sectPr.
const usablePageWidthTwips = 9360

func writeTable(b *bytes.Buffer, t *Table) {
	cols := tableColumnCount(t)
	widths := equalColumnWidths(cols, usablePageWidthTwips)

	b.WriteString(`<w:tbl>`)
	b.WriteString(`<w:tblPr>` +
		`<w:tblStyle w:val="TableGrid"/>` +
		// Prefer fixed layout with an explicit grid; w:tcW="0" auto collapses
		// columns to near-zero width in Word desktop.
		`<w:tblW w:w="5000" w:type="pct"/>` +
		`<w:tblLayout w:type="fixed"/>` +
		`</w:tblPr>`)

	b.WriteString(`<w:tblGrid>`)
	for _, w := range widths {
		fmt.Fprintf(b, `<w:gridCol w:w="%d"/>`, w)
	}
	b.WriteString(`</w:tblGrid>`)

	for _, row := range t.rows {
		b.WriteString(`<w:tr>`)
		if row.isHdr {
			b.WriteString(`<w:trPr><w:tblHeader/></w:trPr>`)
		}
		for i, cell := range row.cells {
			cw := widths[0]
			if i < len(widths) {
				cw = widths[i]
			}
			fmt.Fprintf(b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, cw)
			if row.isHdr {
				b.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="D9D9D9"/>`)
			}
			b.WriteString(`</w:tcPr>`)
			writePara(b, cell)
			b.WriteString(`</w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl>` + "\n")
}

func tableColumnCount(t *Table) int {
	n := 0
	for _, row := range t.rows {
		if len(row.cells) > n {
			n = len(row.cells)
		}
	}
	return n
}

// equalColumnWidths splits totalTwips across cols as evenly as possible.
func equalColumnWidths(cols, totalTwips int) []int {
	if cols < 1 {
		return nil
	}
	base := totalTwips / cols
	rem := totalTwips % cols
	out := make([]int, cols)
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// styleID converts "Heading 1" → "Heading1" (Word style IDs have no spaces).
func styleID(name string) string {
	return strings.ReplaceAll(name, " ", "")
}

// xmlAttr escapes a string for use inside an XML attribute value.
func xmlAttr(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func writeEntry(zw *zip.Writer, name string, data []byte) error {
	h := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: fixedZipTime,
	}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return fmt.Errorf("zip entry %q: %w", name, err)
	}
	_, err = w.Write(data)
	return err
}
