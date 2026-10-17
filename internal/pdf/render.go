package pdf

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/theme"
)

// Options configures PDF rendering.
type Options struct {
	Theme *theme.Theme
}

// Render writes a PDF file for doc to outputPath.
func Render(doc *parser.Document, outputPath string, opts Options) error {
	f, err := os.Create(outputPath) //nolint:gosec // path is caller-supplied
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return RenderTo(doc, f, opts)
}

// RenderTo writes a PDF for doc to w.
func RenderTo(doc *parser.Document, w io.Writer, opts Options) error {
	th := opts.Theme
	if th == nil {
		th = theme.Default()
	}
	r := &walker{
		source:       doc.Source,
		theme:        th,
		canvas:       newCanvas(th),
		unknownKinds: map[string]bool{},
		footnotes:    map[int]string{},
	}
	r.collectFootnotes(doc.Node)
	if err := ast.Walk(doc.Node, r.visit); err != nil {
		return err
	}
	if err := r.errUnknown(); err != nil {
		return err
	}
	r.emitFootnotes()
	r.canvas.addPageNumbers()
	return writePDF(w, r.canvas.contentPages(), r.canvas.pageAnnots())
}

type walker struct {
	source []byte
	theme  *theme.Theme
	canvas *canvas

	boldCount   int
	italicCount int
	strikeCount int

	linkURI   string // absolute URI while inside a Link
	linkDepth int

	listStack       []listState
	listItemDepth   int
	listPrefix      string // bullet/number for the current list item
	blockquoteDepth int

	// Paragraph / heading assembly
	spans  []textSpan
	inPara bool

	// Title block: leading H1 through the first thematic break (centered).
	titleMode titleMode

	// Tables
	tableRows   [][]string
	tableRow    []string
	tableCell   strings.Builder
	inTableCell bool

	// Footnotes: index → body text; refs emit [n]
	footnoteOrder []int
	footnotes     map[int]string
	inFootnote    bool
	fnIndex       int
	fnBuf         strings.Builder

	unknownKinds map[string]bool
}

type listState struct {
	ordered bool
	index   int // next number for ordered lists
}

// titleMode tracks a centered lead-in for documents that open with H1.
type titleMode int

const (
	titlePending titleMode = iota // waiting for first H1
	titleActive                   // centering until first HR / non-title block
	titleDone
)

func (w *walker) errUnknown() error {
	if len(w.unknownKinds) == 0 {
		return nil
	}
	kinds := make([]string, 0, len(w.unknownKinds))
	for k := range w.unknownKinds {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return fmt.Errorf("unhandled markdown nodes: %s", strings.Join(kinds, ", "))
}

func (w *walker) collectFootnotes(root ast.Node) {
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:gosec
		if fn, ok := n.(*extast.Footnote); ok && entering {
			if _, exists := w.footnotes[fn.Index]; !exists {
				w.footnoteOrder = append(w.footnoteOrder, fn.Index)
				w.footnotes[fn.Index] = ""
			}
		}
		return ast.WalkContinue, nil
	})
}

func (w *walker) visit(node ast.Node, entering bool) (ast.WalkStatus, error) {
	switch n := node.(type) {
	case *ast.Document, *ast.TextBlock:

	case *ast.Heading:
		if entering {
			if w.titleMode == titlePending {
				if n.Level == 1 {
					w.titleMode = titleActive
				} else {
					w.titleMode = titleDone
				}
			}
			w.beginBlock()
		} else {
			size := w.theme.HeadingPtAt(n.Level)
			if w.titleMode == titleActive && n.Level == 1 {
				size += 2
			}
			for i := range w.spans {
				w.spans[i].font = fontBold
				if n.Level >= 6 {
					w.spans[i].font = fontItalic
				}
				w.spans[i].size = size
			}
			before := float64(8 + (6-n.Level)*2)
			after := 6.0
			if w.titleMode == titleActive {
				if n.Level == 1 {
					before = 4
					after = 10
				} else {
					before = 4
					after = 6
				}
			}
			w.canvas.moveDown(before)
			w.canvas.writeParagraphAlign(w.spans, 0, after, w.titleMode == titleActive)
			w.spans = nil
			w.inPara = false
		}

	case *ast.Paragraph:
		if entering {
			if w.inFootnote {
				if w.fnBuf.Len() > 0 {
					w.fnBuf.WriteByte(' ')
				}
				break
			}
			// Loose list items: keep the bullet/number already buffered.
			if w.listItemDepth == 0 || len(w.spans) == 0 {
				w.beginBlock()
				if w.listPrefix != "" {
					w.addSpan(w.listPrefix, fontRegular, w.theme.BodyPt, false)
					w.listPrefix = "" // only first paragraph of the item
				}
			}
		} else {
			if w.inFootnote {
				break
			}
			indent := 0.0
			if w.blockquoteDepth > 0 {
				indent = 36 * float64(w.blockquoteDepth)
				for i := range w.spans {
					if w.spans[i].font == fontRegular {
						w.spans[i].font = fontItalic
					}
				}
			}
			after := 8.0
			if w.listItemDepth > 0 {
				indent = 18 * float64(w.listItemDepth)
				after = 4
			}
			center := false
			if w.titleMode == titleActive && w.listItemDepth == 0 && w.blockquoteDepth == 0 {
				allItalic := len(w.spans) > 0
				for _, sp := range w.spans {
					if sp.font != fontItalic && strings.TrimSpace(sp.text) != "" {
						allItalic = false
						break
					}
				}
				if allItalic {
					center = true
					for i := range w.spans {
						if w.spans[i].size == 0 || w.spans[i].size == w.theme.BodyPt {
							w.spans[i].size = w.theme.BodyPt + 1
						}
					}
					after = 10
				} else {
					w.endTitleBlock()
				}
			}
			w.canvas.writeParagraphAlign(w.spans, indent, after, center)
			w.spans = nil
			w.inPara = false
		}

	case *ast.Blockquote:
		if entering {
			w.endTitleBlock()
			w.blockquoteDepth++
		} else {
			w.blockquoteDepth--
		}

	case *ast.ThematicBreak:
		if entering {
			w.canvas.writeHRule()
			w.endTitleBlock()
		}

	case *ast.FencedCodeBlock:
		if entering {
			w.endTitleBlock()
			w.canvas.writeCodeBlock(codeLines(n.Lines(), w.source))
		}
		return ast.WalkSkipChildren, nil

	case *ast.CodeBlock:
		if entering {
			w.endTitleBlock()
			w.canvas.writeCodeBlock(codeLines(n.Lines(), w.source))
		}
		return ast.WalkSkipChildren, nil

	case *ast.HTMLBlock:
		if entering {
			var sb strings.Builder
			for i := 0; i < n.Lines().Len(); i++ {
				seg := n.Lines().At(i)
				sb.Write(seg.Value(w.source))
			}
			if t := stripTags(sb.String()); t != "" {
				w.canvas.writeParagraph([]textSpan{{text: t, font: fontRegular, size: w.theme.BodyPt}}, 0, 8)
			}
		}
		return ast.WalkSkipChildren, nil

	case *ast.List:
		if entering {
			w.endTitleBlock()
			w.listStack = append(w.listStack, listState{ordered: n.IsOrdered(), index: n.Start})
		} else if len(w.listStack) > 0 {
			w.listStack = w.listStack[:len(w.listStack)-1]
		}

	case *ast.ListItem:
		if entering {
			w.listItemDepth++
			prefix := "- "
			if len(w.listStack) > 0 && w.listStack[len(w.listStack)-1].ordered {
				ls := &w.listStack[len(w.listStack)-1]
				prefix = strconv.Itoa(ls.index) + ". "
				ls.index++
			}
			w.listPrefix = prefix
			w.beginBlock()
			w.addSpan(prefix, fontRegular, w.theme.BodyPt, false)
		} else {
			// Tight items (TextBlock, no Paragraph) flush here.
			if len(w.spans) > 0 {
				indent := 18 * float64(w.listItemDepth)
				w.canvas.writeParagraph(w.spans, indent, 4)
				w.spans = nil
			}
			w.inPara = false
			w.listPrefix = ""
			w.listItemDepth--
		}

	case *extast.Table:
		if entering {
			w.endTitleBlock()
			w.tableRows = nil
		} else {
			w.canvas.writeTable(w.tableRows, true)
			w.tableRows = nil
		}

	case *extast.TableHeader:
		if entering {
			w.tableRow = nil
		} else {
			w.tableRows = append(w.tableRows, w.tableRow)
			w.tableRow = nil
		}

	case *extast.TableRow:
		if entering {
			w.tableRow = nil
		} else {
			w.tableRows = append(w.tableRows, w.tableRow)
			w.tableRow = nil
		}

	case *extast.TableCell:
		if entering {
			w.inTableCell = true
			w.tableCell.Reset()
		} else {
			w.tableRow = append(w.tableRow, w.tableCell.String())
			w.inTableCell = false
		}

	case *ast.Emphasis:
		if entering {
			if n.Level == 1 {
				w.italicCount++
			} else {
				w.boldCount++
			}
		} else {
			if n.Level == 1 {
				w.italicCount--
			} else {
				w.boldCount--
			}
		}

	case *extast.Strikethrough:
		if entering {
			w.strikeCount++
		} else {
			w.strikeCount--
		}

	case *extast.TaskCheckBox:
		if entering {
			if n.IsChecked {
				w.addText("[x] ")
			} else {
				w.addText("[ ] ")
			}
		}
		return ast.WalkSkipChildren, nil

	case *ast.Text:
		if entering {
			t := string(n.Segment.Value(w.source))
			w.addText(t)
			if n.HardLineBreak() {
				w.addText("\n")
			} else if n.SoftLineBreak() {
				w.addText(" ")
			}
		}

	case *ast.String:
		if entering {
			w.addText(string(n.Value))
		}

	case *ast.CodeSpan:
		if entering {
			w.addSpan(string(n.Text(w.source)), fontMono, w.theme.CodePt, false)
		}
		return ast.WalkSkipChildren, nil

	case *ast.Link:
		if entering {
			if w.linkDepth == 0 {
				w.linkURI = absoluteURI(string(n.Destination))
			}
			w.linkDepth++
		} else {
			w.linkDepth--
			if w.linkDepth == 0 {
				w.linkURI = ""
			}
		}

	case *ast.AutoLink:
		if entering {
			url := string(n.URL(w.source))
			uri := absoluteURI(url)
			saved := w.linkURI
			w.linkURI = uri
			w.addText(url)
			w.linkURI = saved
		}
		return ast.WalkSkipChildren, nil

	case *ast.Image:
		if entering {
			alt := extractText(n, w.source)
			if alt == "" {
				alt = string(n.Destination)
			}
			w.addSpan("[image: "+alt+"]", fontItalic, w.theme.BodyPt, false)
		}
		return ast.WalkSkipChildren, nil

	case *ast.RawHTML:
		if entering {
			var raw []byte
			for i := 0; i < n.Segments.Len(); i++ {
				seg := n.Segments.At(i)
				raw = append(raw, seg.Value(w.source)...)
			}
			if t := stripTags(string(raw)); t != "" {
				w.addText(t)
			}
		}
		return ast.WalkSkipChildren, nil

	case *extast.FootnoteLink:
		if entering {
			w.addSpan("["+strconv.Itoa(n.Index)+"]", fontRegular, 9, false)
		}
		return ast.WalkSkipChildren, nil

	case *extast.FootnoteList:

	case *extast.Footnote:
		if entering {
			w.inFootnote = true
			w.fnIndex = n.Index
			w.fnBuf.Reset()
		} else {
			w.footnotes[w.fnIndex] = strings.TrimSpace(w.fnBuf.String())
			w.inFootnote = false
		}

	case *extast.FootnoteBacklink:
		return ast.WalkSkipChildren, nil

	default:
		if entering {
			w.unknownKinds[node.Kind().String()] = true
		}
	}
	return ast.WalkContinue, nil
}

func (w *walker) beginBlock() {
	w.spans = nil
	w.inPara = true
}

func (w *walker) endTitleBlock() {
	if w.titleMode != titleDone {
		w.titleMode = titleDone
	}
}

func (w *walker) addText(t string) {
	if t == "" {
		return
	}
	if w.inTableCell {
		w.tableCell.WriteString(t)
		return
	}
	if w.inFootnote {
		w.fnBuf.WriteString(t)
		return
	}
	f := fontRegular
	switch {
	case w.boldCount > 0 && w.italicCount > 0:
		f = fontBold // BoldOblique not registered; prefer bold
	case w.boldCount > 0:
		f = fontBold
	case w.italicCount > 0:
		f = fontItalic
	}
	w.addSpan(t, f, w.theme.BodyPt, w.strikeCount > 0)
}

func (w *walker) addSpan(t string, f fontID, size float64, strike bool) {
	if t == "" {
		return
	}
	if w.inTableCell {
		w.tableCell.WriteString(t)
		return
	}
	if w.inFootnote {
		w.fnBuf.WriteString(t)
		return
	}
	if !w.inPara && w.listItemDepth == 0 {
		w.beginBlock()
	}
	w.spans = append(w.spans, textSpan{
		text: t, font: f, size: size, strike: strike, uri: w.linkURI,
	})
}

// absoluteURI returns a URI safe for PDF /URI actions. Relative paths are
// rejected (empty) so viewers do not open them as local files. Bare www.
// hosts and scheme-relative URLs are normalized to https.
func absoluteURI(dest string) string {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return ""
	}
	lower := strings.ToLower(dest)
	switch {
	case strings.HasPrefix(lower, "https://"),
		strings.HasPrefix(lower, "http://"),
		strings.HasPrefix(lower, "mailto:"):
		return dest
	case strings.HasPrefix(dest, "//"):
		return "https:" + dest
	case strings.HasPrefix(lower, "www."):
		return "https://" + dest
	case strings.Contains(dest, "://"):
		// Other absolute schemes (rare in GFM); pass through.
		return dest
	default:
		return ""
	}
}

func (w *walker) emitFootnotes() {
	if len(w.footnoteOrder) == 0 {
		return
	}
	w.canvas.moveDown(12)
	w.canvas.writeHRule()
	w.canvas.writeParagraph([]textSpan{{text: "Notes", font: fontBold, size: 14}}, 0, 8)
	for _, idx := range w.footnoteOrder {
		body := w.footnotes[idx]
		w.canvas.writeParagraph([]textSpan{{
			text: "[" + strconv.Itoa(idx) + "] " + body,
			font: fontRegular,
			size: 10,
		}}, 0, 4)
	}
}

func codeLines(lines *text.Segments, source []byte) []string {
	out := make([]string, 0, lines.Len())
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		s := string(seg.Value(source))
		s = strings.TrimRight(s, "\n\r")
		out = append(out, s)
	}
	return out
}

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

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
