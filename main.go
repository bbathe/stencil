package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bbathe/stencil/internal/converter"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const usage = `stencil — convert GitHub Flavored Markdown to .docx or .pdf

Usage:
  stencil [flags] INPUT.md
  stencil INPUT.md [flags]
  stencil -                              # read stdin, write stdout
  stencil - -output out.docx             # read stdin, write file
  stencil doc.md -output doc.pdf         # native PDF (built-in fonts)
  stencil doc.md -style theme.json       # shared margins/fonts/sizes

Flags:
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}

	var (
		outputPath  string
		stylePath   string
		showVersion bool
	)

	flag.StringVar(&outputPath, "output", "", "output path (default: input with .docx extension; \"-\" for stdout; use .pdf for PDF)")
	flag.StringVar(&stylePath, "style", "", "JSON theme file (margins, fonts, sizes) for docx and PDF")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")

	// Go's flag package stops at the first non-flag argument, so
	// `stencil README.md --output foo.docx` would leave --output unparsed.
	// Fix: split os.Args into flags and positional args, put flags first,
	// then hand the reordered slice to flag.Parse.
	flags, positional := splitArgs(os.Args[1:])
	if err := flag.CommandLine.Parse(append(flags, positional...)); err != nil {
		os.Exit(2)
	}

	if showVersion {
		fmt.Printf("stencil %s\n", version)
		os.Exit(0)
	}

	// After reordering, flag.Args() holds only the positional args.
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := args[0]

	if outputPath == "" {
		if inputPath == "-" {
			outputPath = "-"
		} else {
			outputPath = replaceExt(inputPath, ".docx")
		}
	} else if outputPath != "-" {
		// If --output is a directory, place the default .docx inside it.
		if fi, err := os.Stat(outputPath); err == nil && fi.IsDir() {
			base := "stdin.docx"
			if inputPath != "-" {
				base = replaceExt(filepath.Base(inputPath), ".docx")
			}
			outputPath = filepath.Join(outputPath, base)
		} else {
			lower := strings.ToLower(outputPath)
			switch {
			case strings.HasSuffix(lower, ".docx"), strings.HasSuffix(lower, ".pdf"):
				// keep as-is
			default:
				outputPath += ".docx"
			}
		}
	}

	opts := converter.Options{
		InputPath:  inputPath,
		OutputPath: outputPath,
		StylePath:  stylePath,
	}

	if err := converter.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "stencil: %v\n", err)
		os.Exit(1)
	}

	if outputPath != "-" {
		fmt.Printf("wrote %s\n", outputPath)
	}
}

// splitArgs separates flag arguments from positional arguments so that
// flags may appear before or after the input file.
//
// It understands which of stencil's own flags take a value so it can
// correctly pair e.g. ["--output", "foo.docx"] as a single flag entry.
func splitArgs(args []string) (flags, positional []string) {
	valueFlags := map[string]bool{
		"-output":  true,
		"--output": true,
		"-style":   true,
		"--style":  true,
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			i++
			continue
		}

		if arg == "-" {
			positional = append(positional, arg)
			i++
			continue
		}

		if strings.Contains(arg, "=") {
			flags = append(flags, arg)
			i++
			continue
		}

		if !valueFlags[arg] {
			flags = append(flags, arg)
			i++
			continue
		}

		flags = append(flags, arg)
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i += 2
		} else {
			i++
		}
	}
	return
}

// replaceExt replaces the file extension of path with ext.
// If path has no extension, ext is appended.
func replaceExt(path, ext string) string {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	if i := strings.LastIndex(base, "."); i >= 0 {
		base = base[:i]
	}

	return filepath.Join(dir, base+ext)
}
