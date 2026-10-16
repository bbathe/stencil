# stencil

`stencil` takes a GitHub Flavored Markdown file and produces a `.docx`. Single binary, no runtime dependencies, deterministic output.

Pandoc works, but it's a large install and does far more than most people need. If your use case is "turn this `.md` into a Word doc that matches our template," stencil covers it without the overhead.

## Install

```bash
go install github.com/bbathe/stencil@latest
```

Pre-built binaries (with `SHA256SUMS`) are on the [Releases](https://github.com/bbathe/stencil/releases) page if you'd rather not build from source.

## Usage

```bash
stencil doc.md                                              # outputs doc.docx
stencil doc.md -output reports/out.docx                    # custom path
stencil doc.md -template reference.docx                    # borrow styles.xml
stencil doc.md -template reference.docx -style-map s.json  # custom style mapping
cat doc.md | stencil -                                      # stdin → stdout
stencil - -output out.docx < doc.md                        # stdin → file
stencil -version
```

## Flags

| Flag          | Default               | Description                               |
|---------------|-----------------------|-------------------------------------------|
| `-template`   | *(none)*              | Reference `.docx` to borrow `styles.xml` from (numbering, headers/footers, and margins are not copied) |
| `-output`     | `<input>.docx`        | Output file path, or `-` for stdout       |
| `-style-map`  | *(built-in defaults)* | JSON file mapping elements to style names |
| `-version`    | —                     | Print version and exit                    |

Use `-` as the input path to read Markdown from stdin. When input is `-` and `-output` is omitted, the DOCX is written to stdout.

## Templates and style maps

Without `-template`, built-in styles use named fonts (**Arial** body/headings, **Courier New** code) so Word and LibreOffice render consistently — no Office theme font references.

Pass a `-template` and stencil pulls the `styles.xml` out of that document and injects it into a fresh one. Your headings, body text, and code blocks all pick up whatever styles are defined there. The source document itself is never touched. Other template parts (theme, numbering, headers/footers, section margins) are not borrowed. Templates that rely on theme fonts may still look wrong in LibreOffice; prefer templates with named fonts.

If you need to rename styles (say your template calls it "Body Text" instead of "Normal"), write a style map:

```json
{
  "heading1":   "Heading 1",
  "heading2":   "Heading 2",
  "heading3":   "Heading 3",
  "heading4":   "Heading 4",
  "heading5":   "Heading 5",
  "heading6":   "Heading 6",
  "body":       "Normal",
  "code_block": "Code",
  "blockquote": "Quote",
  "list_item":  "List Paragraph"
}
```

Keys are fixed; values map to whatever your template calls the style.

## What it converts

- Headings H1–H6
- Paragraphs, bold, italic, bold+italic, strikethrough
- Soft line breaks (joined with a space) and hard line breaks
- Code spans and fenced code blocks (with syntax highlighting)
- Blockquotes (nested depth preserved; uses the `blockquote` style)
- Ordered and unordered lists, including loose items and custom start numbers (`3. …`)
- Task list checkboxes (`- [x]` / `- [ ]`)
- Tables (GFM), including column alignment
- Horizontal rules
- Hyperlinks and autolinks (emphasis, strikethrough, and code spans inside link labels are preserved)
- Local images (PNG, JPEG, GIF, WebP); paths resolve relative to the Markdown file
- Footnotes (with inline formatting — bold, italic, links — in the footnote body)
- Inline and block raw HTML (tags stripped, text preserved)

Remote image URLs, `data:` URIs, and SVG are not embedded; the alt text (or path) is shown in italics instead.

Output is byte-stable for the same input: timestamps in the package and zip entries are fixed.

## License

BSD 3-Clause — see [LICENSE](LICENSE).
