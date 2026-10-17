# stencil

`stencil` takes a GitHub Flavored Markdown file and produces a `.docx` or a simple native `.pdf`. Single binary, no runtime dependencies, deterministic output.

Pandoc works, but it's a large install and does far more than most people need. If your use case is "turn this `.md` into a Word doc or PDF with sensible defaults," stencil covers it without the overhead.

## Install

```bash
go install github.com/bbathe/stencil@latest
```

Pre-built binaries (with `SHA256SUMS`) are on the [Releases](https://github.com/bbathe/stencil/releases) page if you'd rather not build from source.

## Usage

```bash
stencil doc.md                                              # outputs doc.docx
stencil doc.md -output reports/out.docx                    # custom path
stencil doc.md -output doc.pdf                             # native PDF
cat doc.md | stencil -                                      # stdin → stdout
stencil - -output out.docx < doc.md                        # stdin → file
stencil -version
```

## Flags

| Flag       | Default        | Description                                                    |
|------------|----------------|----------------------------------------------------------------|
| `-output`  | `<input>.docx` | Output file path (`.docx` or `.pdf`), or `-` for stdout        |
| `-version` | —              | Print version and exit                                         |

Use `-` as the input path to read Markdown from stdin. When input is `-` and `-output` is omitted, the DOCX is written to stdout.

## Built-in styles

Both formats use fixed built-in styling (no templates or style maps):

- **DOCX:** Arial body/headings, Courier New for code (named fonts, no Office theme refs)
- **PDF:** Helvetica / Helvetica-Bold / Helvetica-Oblique, Courier for code

## PDF output

`-output file.pdf` writes a PDF with a small hand-rolled writer (no Office, no external tools). Layout is simple GFM: headings, paragraphs, lists, tables, code blocks, quotes, rules, links as text, images as `[image: …]` placeholders, footnotes as an end Notes section. Unicode outside WinAnsi becomes `?`.

## What it converts

- Headings H1–H6
- Paragraphs, bold, italic, bold+italic, strikethrough
- Soft line breaks (joined with a space) and hard line breaks
- Code spans and fenced code blocks (with syntax highlighting in DOCX)
- Blockquotes (nested depth preserved)
- Ordered and unordered lists, including loose items and custom start numbers (`3. …`)
- Task list checkboxes (`- [x]` / `- [ ]`)
- Tables (GFM), including column alignment
- Horizontal rules
- Hyperlinks and autolinks (emphasis, strikethrough, and code spans inside link labels are preserved in DOCX)
- Local images (PNG, JPEG, GIF, WebP) in DOCX; paths resolve relative to the Markdown file
- Footnotes (DOCX footnote part; PDF Notes section)
- Inline and block raw HTML (tags stripped, text preserved)

Remote image URLs, `data:` URIs, and SVG are not embedded; the alt text (or path) is shown in italics instead (DOCX) or as `[image: …]` (PDF).

Output is byte-stable for the same input: timestamps in the package and zip entries are fixed.

## License

BSD 3-Clause — see [LICENSE](LICENSE).
