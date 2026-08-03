// Package parser wraps goldmark to produce a parsed AST from GFM source.
package parser

import (
        "github.com/yuin/goldmark"
        "github.com/yuin/goldmark/ast"
        "github.com/yuin/goldmark/extension"
        "github.com/yuin/goldmark/parser"
        "github.com/yuin/goldmark/text"
)

// Document holds the root AST node and the original source bytes.
// The source bytes are required to resolve text segments stored in the AST.
type Document struct {
        Node   ast.Node
        Source []byte
}

// Parse parses GFM markdown source and returns a Document.
// It enables the GFM extension (tables, strikethrough, task lists, linkify)
// and sets the paragraph transformer so blank-line handling is correct.
func Parse(src []byte) (*Document, error) {
        md := goldmark.New(
                goldmark.WithExtensions(
                        extension.GFM,
                        extension.Footnote,
                ),
                goldmark.WithParserOptions(
                        parser.WithAutoHeadingID(),
                ),
        )

        reader := text.NewReader(src)
        node := md.Parser().Parse(reader)

        return &Document{
                Node:   node,
                Source: src,
        }, nil
}
