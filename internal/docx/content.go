package docx

// ── Paragraph ─────────────────────────────────────────────────────────────────

// Alignment is horizontal text alignment for a paragraph or table cell.
type Alignment int

const (
	AlignDefault Alignment = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// Para is a paragraph under construction.
type Para struct {
	style           string
	items           []paraItem // textRun | hyperlinkItem | imageItem | footnoteRefItem
	numID           int        // 0 = not a list para
	numILvl         int        // 0-based list depth
	blockquoteDepth int        // 0 = not a blockquote; 1+ = nesting depth
	hrule           bool
	forceBold       bool
	align           Alignment
}

// AddParagraph appends a new paragraph with the given Word style name.
func (d *Document) AddParagraph(style string) *Para {
	p := &Para{style: style}
	d.body = append(d.body, p)
	return p
}

// SetNumPr marks the paragraph as a list item.
func (p *Para) SetNumPr(numId, ilvl int) { p.numID = numId; p.numILvl = ilvl }

// SetBlockquoteDepth sets the blockquote nesting depth (1-based).
// Each additional level adds more left indentation.
func (p *Para) SetBlockquoteDepth(depth int) { p.blockquoteDepth = depth }

// SetHRule adds a bottom border to the paragraph (thematic break).
func (p *Para) SetHRule() { p.hrule = true }

// SetAlignment sets horizontal alignment (left/center/right).
func (p *Para) SetAlignment(a Alignment) { p.align = a }

// AddRun appends a plain run.
func (p *Para) AddRun(text string) { p.addRun(textRun{text: text}) }

// AddItalicRun appends an italic run.
func (p *Para) AddItalicRun(text string) { p.addRun(textRun{text: text, italic: true}) }

// AddCodeRun appends a Courier New run (inline code).
func (p *Para) AddCodeRun(text string) { p.addRun(textRun{text: text, code: true}) }

// AddCodeColorRun appends a Courier New run with a colour (syntax highlighting).
func (p *Para) AddCodeColorRun(text, color string) {
	p.addRun(textRun{text: text, code: true, color: color})
}

// AddFormattedRun appends a run with explicit bold/italic/strike flags.
func (p *Para) AddFormattedRun(text string, bold, italic, strike bool) {
	p.addRun(textRun{text: text, bold: bold, italic: italic, strike: strike})
}

// AddBreak adds a hard line-break to the most recent run (plain or hyperlink).
func (p *Para) AddBreak() {
	if len(p.items) == 0 {
		p.items = append(p.items, textRun{hardBreak: true})
		return
	}
	switch r := p.items[len(p.items)-1].(type) {
	case textRun:
		r.hardBreak = true
		p.items[len(p.items)-1] = r
	case hyperlinkItem:
		if len(r.runs) == 0 {
			r.runs = append(r.runs, textRun{hardBreak: true, underline: true, color: "0563C1"})
		} else {
			last := r.runs[len(r.runs)-1]
			last.hardBreak = true
			r.runs[len(r.runs)-1] = last
		}
		p.items[len(p.items)-1] = r
	}
}

// AddHyperlinkRun appends a hyperlink run.  bold/italic/strike carry through
// from the surrounding emphasis context (including formatting inside the link).
func (p *Para) AddHyperlinkRun(text, rId string, bold, italic, strike bool) {
	p.addHyperlinkRun(rId, textRun{
		text:      text,
		bold:      bold,
		italic:    italic,
		strike:    strike,
		underline: true,
		color:     "0563C1",
	})
}

// AddHyperlinkCodeRun appends a monospace hyperlink run (code span inside a link).
func (p *Para) AddHyperlinkCodeRun(text, rId string) {
	p.addHyperlinkRun(rId, textRun{
		text:      text,
		code:      true,
		underline: true,
		color:     "0563C1",
	})
}

func (p *Para) addHyperlinkRun(rId string, r textRun) {
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

// AddImageItem embeds an image run. drawingID must be unique within the document.
func (p *Para) AddImageItem(rId, altText string, widthEMU, heightEMU int64, drawingID int) {
	p.items = append(p.items, imageItem{
		rId: rId, altText: altText,
		widthEMU: widthEMU, heightEMU: heightEMU,
		drawingID: drawingID,
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
	drawingID int
}

type footnoteRefItem struct{ id int }

func (textRun) isParaItem()         {}
func (hyperlinkItem) isParaItem()   {}
func (imageItem) isParaItem()       {}
func (footnoteRefItem) isParaItem() {}
