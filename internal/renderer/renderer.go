// Package renderer walks a goldmark AST and emits a .docx document
// using the internal docx builder (stdlib only — no external dependencies).
package renderer

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/bbathe/stencil/internal/docx"
	"github.com/bbathe/stencil/internal/highlight"
	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/stylemap"
)

// Render walks doc and writes a DOCX file to outputPath.
// If templatePath is non-empty, styles are borrowed from that document.
func Render(doc *parser.Document, outputPath, templatePath string, sm *stylemap.StyleMap) error {
	var (
		d   *docx.Document
		err error
	)

	if templatePath != "" {
		d, err = docx.LoadTemplate(templatePath)
		if err != nil {
			return err
		}
	} else {
		d = docx.New()
	}

	w := &walker{
		doc:                d,
		source:             doc.Source,
		styleMap:           sm,
		footnoteIDs:        make(map[int]int),
		footnoteParagraphs: make(map[int]*docx.Para),
	}

	// Pre-walk: register all footnote definitions so that inline FootnoteLink
	// nodes can resolve their docx ids during the main pass.
	w.collectFootnotes(doc.Node)

	if err := ast.Walk(doc.Node, w.visit); err != nil {
		return fmt.Errorf("walking AST: %w", err)
	}

	return d.SaveToFile(outputPath)
}

// ── Walker state ──────────────────────────────────────────────────────────────

type walker struct {
	doc      *docx.Document
	source   []byte
	styleMap *stylemap.StyleMap

	currentPara *docx.Para // paragraph being built during inline walk

	// Inline formatting counters.  Incremented on Enter, decremented on Leave.
	// Counter approach (vs a stack) naturally handles arbitrary nesting such as
	// **_bold-italic_** or ~~**bold-strike**~~.
	boldCount   int
	italicCount int
	strikeCount int

	// List tracking: one numId per nesting level.
	// Bullet lists share numId 1; each ordered list instance gets its own numId
	// from NewOrderedNumID() so numbering restarts across separate lists.
	listNumIDs []int

	// pendingNumId/Ilvl are set when a ListItem is entered and consumed when
	// the first paragraph (Paragraph node or inline Text) inside it is created.
	// This is necessary because loose list items wrap their content in a
	// Paragraph child, while tight list items have Text as a direct child.
	pendingNumId int // 0 = nothing pending
	pendingILvl  int

	// blockquoteDepth tracks nesting of blockquotes (0 = not inside one).
	blockquoteDepth int

	// Table tracking
	inTable      bool
	currentTable *docx.Table
	currentRow   *docx.Row

	// Footnote tracking.
	// footnoteIDs maps a goldmark footnote Index (1-based) → docx footnote id.
	// footnoteParagraphs maps the same index → the *Para to write content into.
	footnoteIDs        map[int]int
	footnoteParagraphs map[int]*docx.Para

	// inFootnote is true while we are rendering a Footnote definition node.
	// currentPara is redirected to the footnote Para; savedPara restores it on exit.
	inFootnote bool
	savedPara  *docx.Para
}

func (w *walker) listDepth() int { return len(w.listNumIDs) }

// ── Footnote pre-walk ─────────────────────────────────────────────────────────

// collectFootnotes registers all Footnote definition nodes before the main
// rendering pass, so inline FootnoteLink references can resolve their ids.
func (w *walker) collectFootnotes(root ast.Node) {
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:gosec // G104: walker never returns error
		if fn, ok := n.(*extast.Footnote); ok && entering {
			id, para := w.doc.AddFootnote()
			w.footnoteIDs[fn.Index] = id
			w.footnoteParagraphs[fn.Index] = para
		}
		return ast.WalkContinue, nil
	})
}

// ── ensureCurrentPara ─────────────────────────────────────────────────────────

// ensureCurrentPara guarantees w.currentPara is non-nil before an inline node
// adds content.  It is called by all inline-node handlers so that tight list
// items (where Text is a direct child of ListItem, with no Paragraph wrapper)
// still get a properly-numbered paragraph created on demand.
func (w *walker) ensureCurrentPara() {
	if w.currentPara != nil || w.inFootnote {
		return
	}
	if w.pendingNumId > 0 {
		p := w.doc.AddParagraph(w.styleMap.Style("list_item"))
		p.SetNumPr(w.pendingNumId, w.pendingILvl)
		w.pendingNumId = 0
		w.currentPara = p
	}
}

// ── Main visitor ──────────────────────────────────────────────────────────────

func (w *walker) visit(node ast.Node, entering bool) (ast.WalkStatus, error) {
	switch n := node.(type) {

	// ── Document root ─────────────────────────────────────────────────────────

	case *ast.Document:
		// nothing

	// ── Headings ─────────────────────────────────────────────────────────────

	case *ast.Heading:
		if entering {
			key := fmt.Sprintf("heading%d", n.Level)
			p := w.doc.AddParagraph(w.styleMap.Style(key))
			w.currentPara = p
		} else {
			w.currentPara = nil
		}

	// ── Paragraph ─────────────────────────────────────────────────────────────
	//
	// Gap 1 fix: loose list items wrap content in a Paragraph child.  When a
	// pendingNumId is set (i.e. we are inside a ListItem), the new paragraph
	// inherits the list's numId/ilvl instead of being a plain body paragraph.
	//
	// Gap 5 fix: when inFootnote, don't create a new docx Para — just add a
	// space separator and continue writing into the current footnote Para.

	case *ast.Paragraph:
		if entering {
			if w.inFootnote {
				// Multiple paragraphs in one footnote: join with a space.
				if w.currentPara != nil {
					w.currentPara.AddRun(" ")
				}
				// Do NOT change currentPara — it still points at the footnote Para.
				break
			}
			p := w.doc.AddParagraph(w.styleMap.Style("body"))
			if w.pendingNumId > 0 {
				// Loose list item: inherit the pending numPr.
				p.SetNumPr(w.pendingNumId, w.pendingILvl)
				w.pendingNumId = 0
			} else if w.blockquoteDepth > 0 {
				p.SetBlockquoteDepth(w.blockquoteDepth)
			}
			w.currentPara = p
		} else {
			if !w.inFootnote {
				w.currentPara = nil
			}
		}

	// ── Blockquote ────────────────────────────────────────────────────────────
	//
	// Gap 4 fix: track depth so nested blockquotes produce deeper indentation.

	case *ast.Blockquote:
		if entering {
			w.blockquoteDepth++
		} else {
			w.blockquoteDepth--
		}

	// ── Thematic break (horizontal rule) ─────────────────────────────────────

	case *ast.ThematicBreak:
		if entering {
			p := w.doc.AddParagraph(w.styleMap.Style("body"))
			p.SetHRule()
		}

	// ── Code blocks ───────────────────────────────────────────────────────────

	case *ast.FencedCodeBlock:
		if entering {
			w.emitCodeBlock(n.Lines(), string(n.Language(w.source)))
		}
		return ast.WalkSkipChildren, nil

	case *ast.CodeBlock:
		if entering {
			w.emitCodeBlock(n.Lines(), "")
		}
		return ast.WalkSkipChildren, nil

	// ── Raw HTML blocks ───────────────────────────────────────────────────────
	//
	// Gap 3 fix: strip tags and emit text content so nothing is silently lost.

	case *ast.HTMLBlock:
		if entering {
			var sb strings.Builder
			for i := 0; i < n.Lines().Len(); i++ {
				seg := n.Lines().At(i)
				sb.Write(seg.Value(w.source))
			}
			if text := stripHTMLTags(sb.String()); text != "" {
				p := w.doc.AddParagraph(w.styleMap.Style("body"))
				p.AddRun(text)
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Lists ─────────────────────────────────────────────────────────────────

	case *ast.List:
		if entering {
			var numId int
			if n.IsOrdered() {
				numId = w.doc.NewOrderedNumID()
			} else {
				numId = w.doc.BulletNumID()
			}
			w.listNumIDs = append(w.listNumIDs, numId)
		} else {
			if len(w.listNumIDs) > 0 {
				w.listNumIDs = w.listNumIDs[:len(w.listNumIDs)-1]
			}
		}

	// Gap 1 fix (continued): don't create the paragraph here; set pending state
	// so it is created on demand by ensureCurrentPara() or the Paragraph handler.

	case *ast.ListItem:
		if entering {
			depth := w.listDepth()
			ilvl := max(depth-1, 0)
			w.pendingNumId = w.listNumIDs[depth-1]
			w.pendingILvl = ilvl
		} else {
			w.currentPara = nil
			w.pendingNumId = 0
		}

	// ── Tables ────────────────────────────────────────────────────────────────

	case *extast.Table:
		if entering {
			t := w.doc.AddTable()
			w.currentTable = t
			w.inTable = true
		} else {
			w.inTable = false
			w.currentTable = nil
		}

	case *extast.TableHeader:
		if entering && w.currentTable != nil {
			w.currentRow = w.currentTable.AddRow(true)
		} else {
			w.currentRow = nil
		}

	case *extast.TableRow:
		if entering && w.currentTable != nil {
			w.currentRow = w.currentTable.AddRow(false)
		} else {
			w.currentRow = nil
		}

	case *extast.TableCell:
		if entering && w.currentRow != nil {
			p := w.currentRow.AddCell(w.styleMap.Style("body"))
			w.currentPara = p
		} else {
			w.currentPara = nil
		}

	// ── Inline: emphasis (bold / italic) ─────────────────────────────────────

	case *ast.Emphasis:
		if entering {
			switch n.Level {
			case 1:
				w.italicCount++
			case 2:
				w.boldCount++
			}
		} else {
			switch n.Level {
			case 1:
				w.italicCount--
			case 2:
				w.boldCount--
			}
		}

	// ── Inline: strikethrough (GFM ~~text~~) ─────────────────────────────────

	case *extast.Strikethrough:
		if entering {
			w.strikeCount++
		} else {
			w.strikeCount--
		}

	// ── Inline: task-list checkbox ────────────────────────────────────────────
	//
	// Gap 2 fix: emit ☑ / ☐ as the first run in the list item paragraph.

	case *extast.TaskCheckBox:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				if n.IsChecked {
					w.currentPara.AddRun("☑ ")
				} else {
					w.currentPara.AddRun("☐ ")
				}
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Inline: plain text ───────────────────────────────────────────────────

	case *ast.Text:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				w.addStyledRun(string(n.Segment.Value(w.source)))
				if n.HardLineBreak() {
					w.currentPara.AddBreak()
				}
			}
		}

	case *ast.String:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				w.addStyledRun(string(n.Value))
			}
		}

	// ── Inline: code span ─────────────────────────────────────────────────────

	case *ast.CodeSpan:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				w.currentPara.AddCodeRun(string(n.Text(w.source)))
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Inline: hyperlink ────────────────────────────────────────────────────

	case *ast.Link:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				url := string(n.Destination)
				rId := w.doc.RegisterHyperlink(url)
				w.currentPara.AddHyperlinkRun(
					extractText(n, w.source), rId,
					w.boldCount > 0, w.italicCount > 0, w.strikeCount > 0,
				)
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Inline: autolink <https://…> ─────────────────────────────────────────

	case *ast.AutoLink:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				url := string(n.URL(w.source))
				rId := w.doc.RegisterHyperlink(url)
				w.currentPara.AddHyperlinkRun(url, rId, false, false, false)
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Inline: image ────────────────────────────────────────────────────────

	case *ast.Image:
		if entering {
			w.ensureCurrentPara()
			if w.currentPara != nil {
				path := string(n.Destination)
				altText := extractText(n, w.source)
				rId, imgW, imgH, regErr := w.doc.RegisterImage(path)
				if regErr != nil {
					w.currentPara.AddItalicRun(fmt.Sprintf("[image: %s]", altText))
				} else {
					w.currentPara.AddImageItem(rId, altText, imgW, imgH)
				}
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Inline: raw HTML span ─────────────────────────────────────────────────
	//
	// Gap 3 fix: strip tags and preserve text content.

	case *ast.RawHTML:
		if entering && w.currentPara != nil {
			var raw []byte
			for i := 0; i < n.Segments.Len(); i++ {
				seg := n.Segments.At(i)
				raw = append(raw, seg.Value(w.source)...)
			}
			if text := stripHTMLTags(string(raw)); text != "" {
				w.addStyledRun(text)
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Footnote: inline reference mark ──────────────────────────────────────

	case *extast.FootnoteLink:
		if entering && w.currentPara != nil {
			if id, ok := w.footnoteIDs[n.Index]; ok {
				w.currentPara.AddFootnoteRef(id)
			}
		}
		return ast.WalkSkipChildren, nil

	// ── Footnote: definition list ─────────────────────────────────────────────
	//
	// Gap 5 fix: walk into the FootnoteList so each Footnote node's content
	// can be rendered into its registered Para (instead of being skipped).

	case *extast.FootnoteList:
		// Fall through — process children.

	// ── Footnote: individual definition ──────────────────────────────────────
	//
	// Gap 5 fix: redirect currentPara into the footnote Para while rendering
	// the footnote body so all inline formatting (bold, italic, links, code)
	// is preserved in the actual footnotes.xml.

	case *extast.Footnote:
		if entering {
			if para, ok := w.footnoteParagraphs[n.Index]; ok {
				w.savedPara = w.currentPara
				w.currentPara = para
				w.inFootnote = true
			}
		} else {
			w.currentPara = w.savedPara
			w.inFootnote = false
		}
	}

	return ast.WalkContinue, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// addStyledRun appends a run to currentPara using the current inline
// formatting counters to determine bold/italic/strike.
func (w *walker) addStyledRun(t string) {
	if w.currentPara == nil || t == "" {
		return
	}
	w.currentPara.AddFormattedRun(t, w.boldCount > 0, w.italicCount > 0, w.strikeCount > 0)
}

// extractText recursively collects the plain-text content of an AST subtree.
// Used to gather link label text, image alt text, and footnote body text.
func extractText(n ast.Node, source []byte) string {
	var sb strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *ast.Text:
			sb.Write(c.Segment.Value(source))
		case *ast.String:
			sb.Write(c.Value)
		default:
			sb.WriteString(extractText(c, source))
		}
	}
	return sb.String()
}

// emitCodeBlock writes each line of a fenced or indented code block as a
// separate Code-style paragraph.  When lang is non-empty, the highlight
// package tokenises the code so keywords and literals receive colour runs.
func (w *walker) emitCodeBlock(lines *text.Segments, lang string) {
	var sb strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		sb.Write(seg.Value(w.source))
	}
	code := strings.TrimRight(sb.String(), "\n")
	if code == "" {
		return
	}

	tokens := highlight.Tokenize(code, lang)
	lineGroups := splitTokensByLine(tokens)

	for _, group := range lineGroups {
		p := w.doc.AddParagraph(w.styleMap.Style("code_block"))
		for _, lt := range group {
			if lt.color != "" {
				p.AddCodeColorRun(lt.text, lt.color)
			} else {
				p.AddCodeRun(lt.text)
			}
		}
	}
}

// lineToken is a coloured text span within a single code line.
type lineToken struct {
	text  string
	color string // 6-digit hex, empty = default (black)
}

// splitTokensByLine splits highlight tokens on embedded newlines so that each
// entry in the result corresponds to exactly one source line.
// Blank lines inside a code block produce a nil (empty) entry so that
// emitCodeBlock still emits a paragraph for them.
func splitTokensByLine(tokens []highlight.Token) [][]lineToken {
	var lines [][]lineToken
	var current []lineToken

	for _, tok := range tokens {
		parts := strings.Split(tok.Text, "\n")
		for i, part := range parts {
			if part != "" {
				current = append(current, lineToken{text: part, color: tok.Color})
			}
			if i < len(parts)-1 {
				lines = append(lines, current)
				current = nil
			}
		}
	}
	if len(current) > 0 {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = append(lines, nil)
	}
	return lines
}

// stripHTMLTags removes all HTML tags from s and returns the remaining text.
// This preserves text content in raw-HTML nodes so nothing is silently lost.
func stripHTMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case !inTag:
			sb.WriteByte(c)
		}
	}
	return strings.TrimSpace(sb.String())
}
