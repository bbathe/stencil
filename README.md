# stencil

`stencil` takes a GitHub Flavored Markdown file and produces a `.docx`. Single binary, no runtime dependencies, deterministic output.

Pandoc works, but it's a large install and does far more than most people need. If your use case is "turn this `.md` into a Word doc with sensible defaults," stencil covers it without the overhead.

## Install

```bash
go install github.com/bbathe/stencil@latest
```

Pre-built binaries (with `SHA256SUMS`) are on the [Releases](https://github.com/bbathe/stencil/releases) page if you'd rather not build from source.

## Usage

```bash
stencil doc.md                                              # outputs doc.docx
stencil doc.md -output reports/out.docx                    # custom path
stencil doc.md -style theme.json                           # theme overrides
cat doc.md | stencil -                                      # stdin → stdout
stencil - -output out.docx < doc.md                        # stdin → file
stencil -version
```

## Flags

| Flag       | Default        | Description                                             |
|------------|----------------|---------------------------------------------------------|
| `-output`  | `<input>.docx` | Output file path, or `-` for stdout                     |
| `-style`   | *(built-in)*   | JSON theme file (margins, fonts, sizes)                 |
| `-version` | —              | Print version and exit                                  |

Use `-` as the input path to read Markdown from stdin. When input is `-` and `-output` is omitted, the DOCX is written to stdout.

## Theme (`-style`)

Optional JSON controls layout knobs. Omitted keys keep defaults:

```json
{
  "margin_in": 1.0,
  "body_pt": 12,
  "code_pt": 10,
  "heading_pt": [24, 18, 14, 12, 12, 12],
  "font": "Arial",
  "mono": "Courier New"
}
```

`margin_in` may be a single number (all sides) or per-side:

```json
"margin_in": { "top": 1.0, "right": 0.75, "bottom": 1.0, "left": 0.5 }
```

Omitted sides keep the default (1.0 in). Each side must be between 0.25 and 2.0.

`font` / `mono` are used as named fonts in the document. Defaults without `-style`: Arial body/headings, Courier New for code.

## What it converts

- Headings H1–H6
- Paragraphs, bold, italic, bold+italic, strikethrough
- Soft line breaks (joined with a space) and hard line breaks
- Code spans and fenced code blocks (with syntax highlighting)
- Blockquotes (nested depth preserved)
- Ordered and unordered lists, including loose items and custom start numbers (`3. …`)
- Task list checkboxes (`- [x]` / `- [ ]`)
- Tables (GFM), including column alignment
- Horizontal rules
- Hyperlinks and autolinks (emphasis, strikethrough, and code spans inside link labels are preserved)
- Local images (PNG, JPEG, GIF, WebP); paths resolve relative to the Markdown file
- Footnotes
- Inline and block raw HTML (tags stripped, text preserved)

Remote image URLs, `data:` URIs, and SVG are not embedded; the alt text (or path) is shown in italics instead.

Output is byte-stable for the same input: timestamps in the package and zip entries are fixed.

## License

BSD 3-Clause — see [LICENSE](LICENSE).
