package pdf

import (
	"fmt"
	"strings"

	"github.com/bbathe/stencil/internal/theme"
)

const (
	pageW = 612.0
	pageH = 792.0
)

type fontID string

const (
	fontRegular fontID = "F1"
	fontBold    fontID = "F2"
	fontItalic  fontID = "F3"
	fontMono    fontID = "F4"
)

// linkAnnot is a clickable URI rectangle on a page (PDF user space).
type linkAnnot struct {
	x1, y1, x2, y2 float64
	uri            string
}

type canvas struct {
	pages   []strings.Builder
	annots  [][]linkAnnot // parallel to pages
	cur     *strings.Builder
	y       float64
	xLeft   float64
	maxW    float64
	marginB float64
	marginT float64
	lineGap float64
	bodyPt  float64
	codePt  float64
}

func newCanvas(th *theme.Theme) *canvas {
	if th == nil {
		th = theme.Default()
	}
	top, right, bottom, left := th.MarginPt()
	c := &canvas{
		xLeft:   left,
		maxW:    pageW - left - right,
		marginB: bottom,
		marginT: top,
		lineGap: 1.25,
		bodyPt:  th.BodyPt,
		codePt:  th.CodePt,
	}
	// Alias table: PDF always draws Helvetica/Courier built-ins.
	_, _ = theme.NormalizePDFFonts(th.Font, th.Mono)
	c.newPage()
	return c
}

func (c *canvas) newPage() {
	c.pages = append(c.pages, strings.Builder{})
	c.annots = append(c.annots, nil)
	c.cur = &c.pages[len(c.pages)-1]
	c.y = pageH - c.marginT
}

func (c *canvas) ensureSpace(h float64) {
	if c.y-h < c.marginB {
		c.newPage()
	}
}

func (c *canvas) moveDown(h float64) {
	c.y -= h
	if c.y < c.marginB {
		c.newPage()
	}
}

func (c *canvas) contentPages() []string {
	out := make([]string, len(c.pages))
	for i := range c.pages {
		out[i] = c.pages[i].String()
	}
	return out
}

func (c *canvas) pageAnnots() [][]linkAnnot {
	return c.annots
}

func (c *canvas) widthsFor(f fontID) *[256]int {
	switch f {
	case fontBold:
		return &helveticaBoldWidths
	case fontMono:
		return &courierWidths
	default:
		return &helveticaWidths
	}
}

func (c *canvas) measure(f fontID, size float64, s string) float64 {
	return stringWidth(c.widthsFor(f), size, s)
}

// textSpan is a styled fragment within a paragraph.
type textSpan struct {
	text   string
	font   fontID
	size   float64
	strike bool
	uri    string // absolute http(s)/mailto URI for link annotations; empty = none
}

type piece struct {
	font   fontID
	size   float64
	text   string
	strike bool
	uri    string
}

func flattenSpans(spans []textSpan, defaultSize float64) []piece {
	var out []piece
	for _, sp := range spans {
		if sp.text == "" {
			continue
		}
		size := sp.size
		if size == 0 {
			size = defaultSize
		}
		s := sp.text
		for len(s) > 0 {
			if s[0] == ' ' || s[0] == '\t' || s[0] == '\n' {
				out = append(out, piece{sp.font, size, " ", sp.strike, sp.uri})
				s = s[1:]
				continue
			}
			i := 0
			for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
				i++
			}
			out = append(out, piece{sp.font, size, s[:i], sp.strike, sp.uri})
			s = s[i:]
		}
	}
	return out
}

func (c *canvas) writeParagraph(spans []textSpan, indent, after float64) {
	words := flattenSpans(spans, c.bodyPt)
	if len(words) == 0 {
		if after > 0 {
			c.moveDown(after)
		}
		return
	}
	size := words[0].size
	lineH := size * c.lineGap
	maxW := c.maxW - indent
	x0 := c.xLeft + indent

	for len(words) > 0 {
		c.ensureSpace(lineH)
		var line []piece
		var lineW float64
		for len(words) > 0 {
			w := c.measure(words[0].font, words[0].size, words[0].text)
			if len(line) > 0 && lineW+w > maxW {
				break
			}
			line = append(line, words[0])
			lineW += w
			words = words[1:]
		}
		for len(line) > 0 && line[0].text == " " {
			line = line[1:]
		}
		if len(line) == 0 {
			if len(words) == 0 {
				break
			}
			line = []piece{words[0]}
			words = words[1:]
		}
		c.emitLine(x0, line)
		c.moveDown(lineH)
	}
	if after > 0 {
		c.moveDown(after)
	}
}

func (c *canvas) emitLine(x float64, line []piece) {
	fmt.Fprintf(c.cur, "BT\n%.2f %.2f Td\n", x, c.y)
	for _, p := range line {
		enc := pdfEncode(p.text)
		if p.uri != "" {
			// Blue link text (RGB).
			c.cur.WriteString("0 0.1 0.8 rg\n")
		}
		fmt.Fprintf(c.cur, "/%s %.2f Tf\n(%s) Tj\n", p.font, p.size, enc)
		if p.uri != "" {
			c.cur.WriteString("0 0 0 rg\n")
		}
		w := c.measure(p.font, p.size, p.text)
		if p.uri != "" {
			pi := len(c.pages) - 1
			c.annots[pi] = append(c.annots[pi], linkAnnot{
				x1: x, y1: c.y - 2, x2: x + w, y2: c.y + p.size*0.2,
				uri: p.uri,
			})
			// Underline
			fmt.Fprintf(c.cur, "ET\n0 0.1 0.8 RG\n0.5 w\n%.2f %.2f m %.2f %.2f l S\n0 0 0 RG\nBT\n%.2f %.2f Td\n",
				x, c.y-1, x+w, c.y-1, x+w, c.y)
			x += w
			continue
		}
		if p.strike {
			y := c.y + p.size*0.3
			fmt.Fprintf(c.cur, "ET\n%.2f %.2f m %.2f %.2f l S\nBT\n%.2f %.2f Td\n",
				x, y, x+w, y, x+w, c.y)
			x += w
			continue
		}
		fmt.Fprintf(c.cur, "%.2f 0 Td\n", w)
		x += w
	}
	c.cur.WriteString("ET\n")
}

func (c *canvas) writeCodeBlock(lines []string) {
	size := c.codePt
	lineH := size * 1.2
	pad := 6.0
	indent := 12.0
	n := len(lines)
	if n == 0 {
		n = 1
	}
	blockH := float64(n)*lineH + pad*2
	c.ensureSpace(blockH)
	x := c.xLeft + indent
	w := c.maxW - indent
	yTop := c.y
	yBot := c.y - blockH
	fmt.Fprintf(c.cur, "0.95 0.95 0.95 rg\n%.2f %.2f %.2f %.2f re f\n0 0 0 rg\n",
		x-pad, yBot, w+pad*2, blockH)
	c.y = yTop - pad - size
	for _, line := range lines {
		c.ensureSpace(lineH)
		fmt.Fprintf(c.cur, "BT\n/%s %.2f Tf\n%.2f %.2f Td\n(%s) Tj\nET\n",
			fontMono, size, x, c.y, pdfEncode(line))
		c.y -= lineH
	}
	c.y = yBot
	c.moveDown(8)
}

func (c *canvas) writeHRule() {
	c.ensureSpace(16)
	y := c.y - 8
	fmt.Fprintf(c.cur, "0.6 0.6 0.6 RG\n0.5 w\n%.2f %.2f m %.2f %.2f l S\n0 0 0 RG\n",
		c.xLeft, y, c.xLeft+c.maxW, y)
	c.moveDown(16)
}

func (c *canvas) writeTable(rows [][]string, header bool) {
	if len(rows) == 0 {
		return
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return
	}
	colW := c.maxW / float64(cols)
	size := c.bodyPt
	lineH := size * 1.3
	for ri, row := range rows {
		for len(row) < cols {
			row = append(row, "")
		}
		c.ensureSpace(lineH + 4)
		y := c.y
		f := fontRegular
		if header && ri == 0 {
			f = fontBold
		}
		for ci := 0; ci < cols; ci++ {
			cell := row[ci]
			for c.measure(f, size, cell) > colW-8 && len(cell) > 1 {
				cell = cell[:len(cell)-1]
			}
			x := c.xLeft + float64(ci)*colW + 4
			fmt.Fprintf(c.cur, "BT\n/%s %.2f Tf\n%.2f %.2f Td\n(%s) Tj\nET\n",
				f, size, x, y-size, pdfEncode(cell))
		}
		fmt.Fprintf(c.cur, "0.7 0.7 0.7 RG\n0.4 w\n%.2f %.2f m %.2f %.2f l S\n0 0 0 RG\n",
			c.xLeft, y-lineH, c.xLeft+c.maxW, y-lineH)
		c.moveDown(lineH + 2)
	}
	c.moveDown(8)
}

// pdfEncode maps runes to WinAnsi where possible and escapes PDF string syntax.
func pdfEncode(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteByte(byte(r))
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		default:
			if r < 128 {
				b.WriteByte(byte(r))
				continue
			}
			if wa := unicodeToWinAnsi(r); wa != 0 {
				if wa == '\\' || wa == '(' || wa == ')' {
					b.WriteByte('\\')
				}
				b.WriteByte(wa)
				continue
			}
			b.WriteByte('?')
		}
	}
	return b.String()
}

func unicodeToWinAnsi(r rune) byte {
	if r >= 0xA0 && r <= 0xFF {
		return byte(r)
	}
	switch r {
	case 0x2013:
		return 0x96
	case 0x2014:
		return 0x97
	case 0x2018, 0x2019:
		return '\''
	case 0x201C, 0x201D:
		return '"'
	case 0x2026:
		return 0x85
	case 0x2022: // bullet
		return 0x95
	case 0x00A0:
		return ' '
	}
	return 0
}
