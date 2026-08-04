// Package docx builds a minimal but complete .docx using only the Go standard
// library (archive/zip + encoding/xml + image).
//
// Supported OOXML parts:
//
//	[Content_Types].xml
//	_rels/.rels
//	docProps/core.xml
//	word/document.xml
//	word/styles.xml
//	word/settings.xml
//	word/numbering.xml
//	word/footnotes.xml      (when footnotes are present)
//	word/media/image*.ext   (when images are present)
//	word/_rels/document.xml.rels
package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Document ──────────────────────────────────────────────────────────────────

// Document accumulates content and serialises it to a .docx file.
type Document struct {
	body   []bodyElement
	styles []byte // raw styles.xml; nil = built-in defaults

	// relationships
	hyperlinks   []hyperlinkEntry
	hyperlinkMap map[string]string // url -> rId (dedup)
	images       []imageEntry

	// list numbering: numId 1 = bullets, 2..N = one per ordered list instance
	orderedListCount int

	// footnotes
	footnotes []footnoteEntry
}

type (
	hyperlinkEntry struct{ rId, url string }
	imageEntry     struct {
		rId, partName, mimeType, ext string
		data                         []byte
	}
)

// footnoteEntry holds the footnote id and a Para for its formatted content.
// The Para is written into during rendering and read back in footnotesXML.
type footnoteEntry struct {
	id   int
	para *Para
}

// New returns an empty Document.
func New() *Document { return &Document{} }

// LoadTemplate opens a reference .docx and borrows its styles.xml.
func LoadTemplate(path string) (*Document, error) {
	f, err := os.Open(path) //nolint:gosec // path is caller-supplied template flag
	if err != nil {
		return nil, fmt.Errorf("opening template %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("reading template zip %q: %w", path, err)
	}

	for _, zf := range zr.File {
		if zf.Name == "word/styles.xml" {
			rc, err := zf.Open()
			if err != nil {
				return nil, fmt.Errorf("reading styles from template: %w", err)
			}
			data, err := io.ReadAll(rc)
			if cerr := rc.Close(); cerr != nil && err == nil {
				err = cerr
			}
			if err != nil {
				return nil, err
			}
			return &Document{styles: data}, nil
		}
	}

	return New(), nil
}

// ── Resource registration ─────────────────────────────────────────────────────

// RegisterHyperlink returns a stable rId for the given URL, creating one if needed.
func (d *Document) RegisterHyperlink(url string) string {
	if d.hyperlinkMap == nil {
		d.hyperlinkMap = make(map[string]string)
	}
	if rId, ok := d.hyperlinkMap[url]; ok {
		return rId
	}
	rId := fmt.Sprintf("rIdHL%d", len(d.hyperlinks)+1)
	d.hyperlinks = append(d.hyperlinks, hyperlinkEntry{rId: rId, url: url})
	d.hyperlinkMap[url] = rId
	return rId
}

// RegisterImage reads the image file at path, stores its bytes, and returns
// (rId, widthEMU, heightEMU).  On error the image is skipped and ("", 0, 0, err)
// is returned so the caller can fall back to alt-text.
func (d *Document) RegisterImage(path string) (string, int64, int64, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied image path
	if err != nil {
		return "", 0, 0, fmt.Errorf("reading image %q: %w", path, err)
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	mime := "image/png"
	switch ext {
	case "jpg", "jpeg":
		mime = "image/jpeg"
		ext = "jpeg"
	case "gif":
		mime = "image/gif"
	default:
		ext = "png"
	}

	n := len(d.images) + 1
	rId := fmt.Sprintf("rIdIM%d", n)
	partName := fmt.Sprintf("word/media/image%d.%s", n, ext)
	d.images = append(d.images, imageEntry{rId: rId, partName: partName, mimeType: mime, ext: ext, data: data})

	// Dimensions in EMU (1 inch = 914400, 96 DPI → 9525 EMU/pixel)
	const emuPerPx = int64(9525)
	const maxW = int64(5943600) // 6.5 inches
	cfg, _, decErr := image.DecodeConfig(bytes.NewReader(data))
	if decErr != nil {
		return rId, 3657600, 2743200, nil // 4" × 3" fallback
	}
	w := int64(cfg.Width) * emuPerPx
	h := int64(cfg.Height) * emuPerPx
	if w > maxW {
		h = h * maxW / w
		w = maxW
	}
	return rId, w, h, nil
}

// AddFootnote registers a new footnote and returns its 1-based id and a *Para
// into which the caller writes the formatted footnote content (runs, hyperlinks, etc.).
func (d *Document) AddFootnote() (int, *Para) {
	id := len(d.footnotes) + 1
	p := &Para{}
	d.footnotes = append(d.footnotes, footnoteEntry{id: id, para: p})
	return id, p
}

// BulletNumID returns the fixed numId for unordered lists.
func (d *Document) BulletNumID() int { return 1 }

// NewOrderedNumID allocates a new numId for an ordered list instance.
// Each separate ordered list must get its own numId so numbering restarts.
func (d *Document) NewOrderedNumID() int {
	d.orderedListCount++
	return 1 + d.orderedListCount // 2, 3, 4, …
}

// ── Paragraph ─────────────────────────────────────────────────────────────────

// Para is a paragraph under construction.
type Para struct {
	style           string
	items           []paraItem // textRun | hyperlinkItem | imageItem | footnoteRefItem
	numID           int        // 0 = not a list para
	numILvl         int        // 0-based list depth
	blockquoteDepth int        // 0 = not a blockquote; 1+ = nesting depth
	hrule           bool
	forceBold       bool
}

// AddParagraph appends a new paragraph with the given Word style name.
func (d *Document) AddParagraph(style string) *Para {
	p := &Para{style: style}
	d.body = append(d.body, p)
	return p
}

// SetNumPr marks the paragraph as a list item.
func (p *Para) SetNumPr(numId, ilvl int) { p.numID = numId; p.numILvl = ilvl }

// SetBlockquote adds a left border to the paragraph (depth 1).
func (p *Para) SetBlockquote() { p.blockquoteDepth = 1 }

// SetBlockquoteDepth sets the blockquote nesting depth (1-based).
// Each additional level adds more left indentation.
func (p *Para) SetBlockquoteDepth(depth int) { p.blockquoteDepth = depth }

// SetHRule adds a bottom border to the paragraph (thematic break).
func (p *Para) SetHRule() { p.hrule = true }

// ForceBold makes every run in this paragraph bold regardless of run props.
func (p *Para) ForceBold() { p.forceBold = true }

// AddRun appends a plain run.
func (p *Para) AddRun(text string) { p.addRun(textRun{text: text}) }

// AddBoldRun appends a bold run.
func (p *Para) AddBoldRun(text string) { p.addRun(textRun{text: text, bold: true}) }

// AddItalicRun appends an italic run.
func (p *Para) AddItalicRun(text string) { p.addRun(textRun{text: text, italic: true}) }

// AddBoldItalicRun appends a bold+italic run.
func (p *Para) AddBoldItalicRun(text string) {
	p.addRun(textRun{text: text, bold: true, italic: true})
}

// AddCodeRun appends a Courier New run (inline code).
func (p *Para) AddCodeRun(text string) { p.addRun(textRun{text: text, code: true}) }

// AddCodeColorRun appends a Courier New run with a colour (syntax highlighting).
func (p *Para) AddCodeColorRun(text, color string) {
	p.addRun(textRun{text: text, code: true, color: color})
}

// AddColorRun appends a coloured run (non-monospace).
func (p *Para) AddColorRun(text, color string) { p.addRun(textRun{text: text, color: color}) }

// AddStrikeRun appends a struck-through run.
func (p *Para) AddStrikeRun(text string) { p.addRun(textRun{text: text, strike: true}) }

// AddFormattedRun appends a run with explicit bold/italic/strike flags.
// This is the general form; the typed helpers above are convenience wrappers.
func (p *Para) AddFormattedRun(text string, bold, italic, strike bool) {
	p.addRun(textRun{text: text, bold: bold, italic: italic, strike: strike})
}

// AddBreak adds a hard line-break to the most recent run.
func (p *Para) AddBreak() {
	if len(p.items) == 0 {
		p.items = append(p.items, textRun{})
		return
	}
	if r, ok := p.items[len(p.items)-1].(textRun); ok {
		r.hardBreak = true
		p.items[len(p.items)-1] = r
	}
}

// AddHyperlinkRun appends a hyperlink run.  bold/italic/strike carry through
// from the surrounding emphasis context.
func (p *Para) AddHyperlinkRun(text, rId string, bold, italic, strike bool) {
	r := textRun{
		text:      text,
		bold:      bold,
		italic:    italic,
		strike:    strike,
		underline: true,
		color:     "0563C1",
	}
	// Coalesce consecutive runs with the same rId into one hyperlinkItem.
	if n := len(p.items); n > 0 {
		if hl, ok := p.items[n-1].(hyperlinkItem); ok && hl.rId == rId {
			hl.runs = append(hl.runs, r)
			p.items[n-1] = hl
			return
		}
	}
	p.items = append(p.items, hyperlinkItem{rId: rId, runs: []textRun{r}})
}

// AddImageItem embeds an image run.
func (p *Para) AddImageItem(rId, altText string, widthEMU, heightEMU int64) {
	p.items = append(p.items, imageItem{
		rId: rId, altText: altText,
		widthEMU: widthEMU, heightEMU: heightEMU,
	})
}

// AddFootnoteRef inserts an inline footnote reference mark.
func (p *Para) AddFootnoteRef(id int) {
	p.items = append(p.items, footnoteRefItem{id: id})
}

func (p *Para) addRun(r textRun) { p.items = append(p.items, r) }

// ── Table ─────────────────────────────────────────────────────────────────────

// Table is a table under construction.
type Table struct{ rows []*Row }

// Row is a table row.
type Row struct {
	cells []*Para
	isHdr bool
}

// AddTable appends a new table.
func (d *Document) AddTable() *Table {
	t := &Table{}
	d.body = append(d.body, t)
	return t
}

// AddRow appends a row.  isHeader=true marks it as a header row (shaded + bold).
func (t *Table) AddRow(isHeader bool) *Row {
	r := &Row{isHdr: isHeader}
	t.rows = append(t.rows, r)
	return r
}

// AddCell appends a cell and returns its embedded paragraph.
// Header rows automatically force-bold their cell paragraph.
func (r *Row) AddCell(style string) *Para {
	p := &Para{style: style}
	if r.isHdr {
		p.forceBold = true
	}
	r.cells = append(r.cells, p)
	return p
}

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
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Appendf(nil, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties
    xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
    xmlns:dc="http://purl.org/dc/elements/1.1/"
    xmlns:dcterms="http://purl.org/dc/terms/"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:creator>stencil</dc:creator>
  <cp:lastModifiedBy>stencil</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, now, now), nil
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

	// ── Nums 2..N: one per ordered-list instance (restart numbering) ──────────
	for i := 0; i < d.orderedListCount; i++ {
		numId := i + 2
		fmt.Fprintf(&b,
			`<w:num w:numId="%d"><w:abstractNumId w:val="1"/>`+
				`<w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride>`+
				`</w:num>`+"\n", numId)
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
	b.WriteString(`<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)

	// Required separator footnotes
	b.WriteString(`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>`)
	b.WriteString(`<w:footnote w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:footnote>`)

	for _, fn := range d.footnotes {
		fmt.Fprintf(&b, `<w:footnote w:id="%d"><w:p>`, fn.id)
		b.WriteString(`<w:pPr><w:pStyle w:val="FootnoteText"/></w:pPr>`)
		b.WriteString(`<w:r><w:rPr><w:vertAlign w:val="superscript"/></w:rPr><w:footnoteRef/></w:r>`)
		b.WriteString(`<w:r><w:t xml:space="preserve"> </w:t></w:r>`)
		// Render the Para items (text runs, hyperlinks, etc.)
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
				}
			}
		}
		b.WriteString(`</w:p></w:footnote>`)
	}

	b.WriteString(`</w:footnotes>`)
	return b.Bytes(), nil
}

// ── XML element writers ───────────────────────────────────────────────────────

func writePara(b *bytes.Buffer, p *Para) {
	b.WriteString(`<w:p>`)

	// Paragraph properties
	needPPr := p.style != "" || p.numID > 0 || p.blockquoteDepth > 0 || p.hrule
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
			b.WriteString(`<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>`)
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

	fmt.Fprintf(b,
		`<w:r><w:rPr/><w:drawing>`+
			`<wp:inline>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:docPr id="1" name="Image" descr="%s"/>`+
			`<a:graphic>`+
			`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic>`+
			`<pic:nvPicPr><pic:cNvPr id="1" name="Image" descr="%s"/><pic:cNvPicPr/></pic:nvPicPr>`+
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
		img.widthEMU, img.heightEMU, altS, altS, img.rId, img.widthEMU, img.heightEMU)
}

func writeTable(b *bytes.Buffer, t *Table) {
	b.WriteString(`<w:tbl>`)
	b.WriteString(`<w:tblPr>` +
		`<w:tblStyle w:val="TableGrid"/>` +
		`<w:tblW w:w="0" w:type="auto"/>` +
		`</w:tblPr>`)

	for _, row := range t.rows {
		b.WriteString(`<w:tr>`)
		if row.isHdr {
			b.WriteString(`<w:trPr><w:tblHeader/></w:trPr>`)
		}
		for _, cell := range row.cells {
			b.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/>`)
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
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("zip entry %q: %w", name, err)
	}
	_, err = w.Write(data)
	return err
}

// ── Internal types ────────────────────────────────────────────────────────────

type bodyElement interface{ isBody() }

func (*Para) isBody()  {}
func (*Table) isBody() {}

type paraItem interface{ isParaItem() }

type textRun struct {
	text      string
	bold      bool
	italic    bool
	code      bool
	strike    bool
	underline bool
	color     string
	hardBreak bool
}

type hyperlinkItem struct {
	rId  string
	runs []textRun
}

type imageItem struct {
	rId       string
	altText   string
	widthEMU  int64
	heightEMU int64
}

type footnoteRefItem struct{ id int }

func (textRun) isParaItem()         {}
func (hyperlinkItem) isParaItem()   {}
func (imageItem) isParaItem()       {}
func (footnoteRefItem) isParaItem() {}

// ── Built-in default styles ───────────────────────────────────────────────────

func defaultStyles() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault><w:rPr>
      <w:rFonts w:asciiTheme="minorHAnsi" w:hAnsiTheme="minorHAnsi"/>
      <w:sz w:val="24"/><w:szCs w:val="24"/>
    </w:rPr></w:rPrDefault>
  </w:docDefaults>

  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr>
  </w:style>

  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="0"/><w:spacing w:before="240" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="48"/><w:szCs w:val="48"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="1"/><w:spacing w:before="200" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="2"/><w:spacing w:before="160" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading4">
    <w:name w:val="heading 4"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="3"/></w:pPr>
    <w:rPr><w:b/><w:i/><w:sz w:val="24"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading5">
    <w:name w:val="heading 5"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="4"/></w:pPr>
    <w:rPr><w:sz w:val="24"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading6">
    <w:name w:val="heading 6"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:outlineLvl w:val="5"/></w:pPr>
    <w:rPr><w:i/><w:sz w:val="24"/></w:rPr>
  </w:style>

  <w:style w:type="paragraph" w:styleId="Code">
    <w:name w:val="Code"/><w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>
      <w:spacing w:after="0"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>
      <w:sz w:val="20"/><w:szCs w:val="20"/>
    </w:rPr>
  </w:style>

  <w:style w:type="paragraph" w:styleId="Quote">
    <w:name w:val="Quote"/><w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:ind w:left="720" w:right="720"/>
      <w:spacing w:before="120" w:after="120"/>
    </w:pPr>
    <w:rPr><w:i/><w:color w:val="555555"/></w:rPr>
  </w:style>

  <w:style w:type="paragraph" w:styleId="ListParagraph">
    <w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:after="80"/></w:pPr>
  </w:style>

  <w:style w:type="paragraph" w:styleId="FootnoteText">
    <w:name w:val="footnote text"/><w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:after="0"/></w:pPr>
    <w:rPr><w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr>
  </w:style>

  <w:style w:type="character" w:styleId="FootnoteReference">
    <w:name w:val="footnote reference"/>
    <w:rPr><w:vertAlign w:val="superscript"/></w:rPr>
  </w:style>

  <w:style w:type="table" w:styleId="TableGrid">
    <w:name w:val="Table Grid"/>
    <w:tblPr><w:tblBorders>
      <w:top    w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:left   w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:bottom w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:right  w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:insideH w:val="single" w:sz="4" w:space="0" w:color="000000"/>
      <w:insideV w:val="single" w:sz="4" w:space="0" w:color="000000"/>
    </w:tblBorders></w:tblPr>
  </w:style>
</w:styles>`)
}
