// Package converter is the top-level coordinator that wires together
// the parser and renderers.
package converter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bbathe/stencil/internal/parser"
	"github.com/bbathe/stencil/internal/pdf"
	"github.com/bbathe/stencil/internal/renderer"
	"github.com/bbathe/stencil/internal/theme"
)

// Options holds all user-supplied conversion parameters.
type Options struct {
	InputPath  string // required: path to the source .md file, or "-" for stdin
	OutputPath string // required: path to write the output, or "-" for stdout
	StylePath  string // optional: JSON theme file (margins, fonts, sizes)
}

// Run executes a full markdown conversion using the given options.
// Output format is inferred from OutputPath extension (.docx or .pdf).
// It returns a non-nil error if any step fails; the error message is
// suitable for printing directly to stderr.
func Run(opts Options) error {
	src, baseDir, err := readInput(opts.InputPath)
	if err != nil {
		return err
	}

	th, err := theme.Load(opts.StylePath)
	if err != nil {
		return err
	}

	doc, err := parser.Parse(src)
	if err != nil {
		return fmt.Errorf("parsing markdown: %w", err)
	}

	if outputFormat(opts.OutputPath) == "pdf" {
		popts := pdf.Options{Theme: th}
		if opts.OutputPath == "-" {
			if err := pdf.RenderTo(doc, os.Stdout, popts); err != nil {
				return fmt.Errorf("rendering pdf: %w", err)
			}
			return nil
		}
		if err := pdf.Render(doc, opts.OutputPath, popts); err != nil {
			return fmt.Errorf("rendering pdf: %w", err)
		}
		return nil
	}

	ropts := renderer.Options{BaseDir: baseDir, Theme: th}
	if opts.OutputPath == "-" {
		if err := renderer.RenderTo(doc, os.Stdout, ropts); err != nil {
			return fmt.Errorf("rendering docx: %w", err)
		}
		return nil
	}
	if err := renderer.Render(doc, opts.OutputPath, ropts); err != nil {
		return fmt.Errorf("rendering docx: %w", err)
	}
	return nil
}

func outputFormat(path string) string {
	if path == "-" {
		return "docx"
	}
	if strings.ToLower(filepath.Ext(path)) == ".pdf" {
		return "pdf"
	}
	return "docx"
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
