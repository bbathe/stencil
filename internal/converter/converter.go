// Package converter is the top-level coordinator that wires together
// the parser, style-map loader, and renderer.
package converter

import (
	"fmt"
	"os"

	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/renderer"
	"github.com/bbathe/stencil/internal/stylemap"
)

// Options holds all user-supplied conversion parameters.
type Options struct {
	InputPath    string // required: path to the source .md file
	OutputPath   string // required: path to write the output .docx
	TemplatePath string // optional: path to a reference .docx
	StyleMapPath string // optional: path to a JSON style-map file
}

// Run executes a full markdown-to-docx conversion using the given options.
// It returns a non-nil error if any step fails; the error message is
// suitable for printing directly to stderr.
func Run(opts Options) error {
	src, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return fmt.Errorf("reading %q: %w", opts.InputPath, err)
	}

	sm, err := stylemap.Load(opts.StyleMapPath)
	if err != nil {
		return err
	}

	doc, err := parser.Parse(src)
	if err != nil {
		return fmt.Errorf("parsing markdown: %w", err)
	}

	if err := renderer.Render(doc, opts.OutputPath, opts.TemplatePath, sm); err != nil {
		return fmt.Errorf("rendering docx: %w", err)
	}

	return nil
}
