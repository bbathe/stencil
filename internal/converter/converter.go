// Package converter is the top-level coordinator that wires together
// the parser, style-map loader, and renderer.
package converter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/renderer"
	"github.com/bbathe/stencil/internal/stylemap"
)

// Options holds all user-supplied conversion parameters.
type Options struct {
	InputPath    string // required: path to the source .md file, or "-" for stdin
	OutputPath   string // required: path to write the output .docx, or "-" for stdout
	TemplatePath string // optional: path to a reference .docx
	StyleMapPath string // optional: path to a JSON style-map file
}

// Run executes a full markdown-to-docx conversion using the given options.
// It returns a non-nil error if any step fails; the error message is
// suitable for printing directly to stderr.
func Run(opts Options) error {
	src, baseDir, err := readInput(opts.InputPath)
	if err != nil {
		return err
	}

	sm, err := stylemap.Load(opts.StyleMapPath)
	if err != nil {
		return err
	}

	doc, err := parser.Parse(src)
	if err != nil {
		return fmt.Errorf("parsing markdown: %w", err)
	}

	if opts.OutputPath == "-" {
		if err := renderer.RenderTo(doc, os.Stdout, opts.TemplatePath, baseDir, sm); err != nil {
			return fmt.Errorf("rendering docx: %w", err)
		}
		return nil
	}

	if err := renderer.Render(doc, opts.OutputPath, opts.TemplatePath, baseDir, sm); err != nil {
		return fmt.Errorf("rendering docx: %w", err)
	}

	return nil
}

// readInput loads markdown bytes and the directory used to resolve relative
// image paths. For stdin ("-"), baseDir is the process working directory.
func readInput(path string) (src []byte, baseDir string, err error) {
	if path == "-" {
		src, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("reading stdin: %w", err)
		}
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return src, "", nil
		}
		return src, wd, nil
	}

	src, err = os.ReadFile(path) //nolint:gosec // path is caller-supplied input flag
	if err != nil {
		return nil, "", fmt.Errorf("reading %q: %w", path, err)
	}
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		return src, filepath.Dir(path), nil
	}
	return src, filepath.Dir(abs), nil
}
