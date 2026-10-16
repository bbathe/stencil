package parser

import (
	"testing"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

func TestParseGFMExtensions(t *testing.T) {
	src := []byte("# H\n\n| a |\n| - |\n| b |\n\n- [x] task\n\n~~x~~\n\nnote[^1]\n\n[^1]: body\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var sawTable, sawTask, sawStrike, sawFootnote bool
	_ = ast.Walk(doc.Node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *extast.Table:
			sawTable = true
		case *extast.TaskCheckBox:
			sawTask = true
		case *extast.Strikethrough:
			sawStrike = true
		case *extast.Footnote:
			sawFootnote = true
		}
		return ast.WalkContinue, nil
	})
	if !sawTable || !sawTask || !sawStrike || !sawFootnote {
		t.Fatalf("extensions missing: table=%v task=%v strike=%v footnote=%v",
			sawTable, sawTask, sawStrike, sawFootnote)
	}
}
