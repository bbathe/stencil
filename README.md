# stencil

`stencil` takes a GitHub Flavored Markdown file and produces a `.docx`. Single binary, no runtime dependencies, deterministic output.

Pandoc works, but it's a large install and does far more than most people need. If your use case is "turn this `.md` into a Word doc that matches our template," stencil covers it without the overhead.

## Install

```bash
go install github.com/bbathe/stencil@latest
```

Pre-built binaries are on the [Releases](https://github.com/bbathe/stencil/releases) page if you'd rather not build from source.

## Usage

```bash
stencil doc.md                                              # outputs doc.docx
stencil doc.md -output reports/out.docx                    # custom path
stencil doc.md -template reference.docx                    # borrow styles
stencil doc.md -template reference.docx -style-map s.json  # custom style mapping
stencil -version
```

## Flags

| Flag          | Default               | Description                               |
|---------------|-----------------------|-------------------------------------------|
| `-template`   | *(none)*              | Reference `.docx` to borrow styles from   |
| `-output`     | `<input>.docx`        | Output file path                          |
| `-style-map`  | *(built-in defaults)* | JSON file mapping elements to style names |
| `-version`    | —                     | Print version and exit                    |

## Templates and style maps

Pass a `-template` and stencil pulls the `styles.xml` out of that document and injects it into a fresh one. Your headings, body text, and code blocks all pick up whatever styles are defined there. The source document itself is never touched.

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
- Paragraphs, bold, italic, bold+italic
- Code spans and fenced code blocks (with syntax highlighting)
- Blockquotes (nested depth preserved)
- Ordered and unordered lists, including loose items
- Task list checkboxes (`- [x]` / `- [ ]`)
- Tables (GFM)
- Horizontal rules and hard line breaks
- Hyperlinks
- Footnotes (with inline formatting — bold, italic, links — in the footnote body)
- Inline and block raw HTML (tags stripped, text preserved)

## License

BSD 3-Clause — see [LICENSE](LICENSE).
