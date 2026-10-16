// Package renderer walks a goldmark AST and emits a .docx document
// using the internal docx builder.
package renderer

import (
	"fmt"
	"io"

	"github.com/yuin/goldmark/ast"

	"github.com/bbathe/stencil/internal/docx"
	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/stylemap"
)

// Options controls how a parsed document is rendered to DOCX.
type Options struct {
	TemplatePath string // optional reference .docx (styles.xml only)
	BaseDir      string // resolve relative image paths against this directory
	StyleMap     *stylemap.StyleMap
}

// Render walks doc and writes a DOCX file to outputPath.
func Render(doc *parser.Document, outputPath string, opts Options) error {
	d, err := build(doc, opts)
	if err != nil {
		return err
	}
	return d.SaveToFile(outputPath)
}

// RenderTo walks doc and writes a DOCX to w.
func RenderTo(doc *parser.Document, w io.Writer, opts Options) error {
	d, err := build(doc, opts)
	if err != nil {
		return err
	}
	return d.Write(w)
}

func build(doc *parser.Document, opts Options) (*docx.Document, error) {
	var (
		d   *docx.Document
		err error
	)

	if opts.TemplatePath != "" {
		d, err = docx.LoadTemplate(opts.TemplatePath)
		if err != nil {
			return nil, err
		}
	} else {
		d = docx.New()
	}
	d.BaseDir = opts.BaseDir

	sm := opts.StyleMap
	if sm == nil {
		sm, err = stylemap.Load("")
		if err != nil {
			return nil, err
		}
	}

	wk := &walker{
		doc:                d,
		source:             doc.Source,
		styleMap:           sm,
		footnoteIDs:        make(map[int]int),
		footnoteParagraphs: make(map[int]*docx.Para),
		unknownKinds:       make(map[string]bool),
	}

	// Pre-walk: register all footnote definitions so that inline FootnoteLink
	// nodes can resolve their docx ids during the main pass.
	wk.collectFootnotes(doc.Node)

	if err := ast.Walk(doc.Node, wk.visit); err != nil {
		return nil, fmt.Errorf("walking AST: %w", err)
	}
	if err := wk.errUnknown(); err != nil {
		return nil, err
	}
	return d, nil
}
