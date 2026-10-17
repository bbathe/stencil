# Word-like Calibri styles

`-style-map` only renames styles; it does **not** set fonts. For Word’s usual look
(Calibri body, Calibri Light headings — what Office uses for theme minor/major),
pass the companion template:

```bash
stencil doc.md \
  -template examples/word-calibri/template.docx \
  -style-map examples/word-calibri/style-map.json
```

The style map matches stencil’s built-in keys to Word’s standard style names (same
as the defaults). The template’s `styles.xml` supplies the Calibri / Calibri Light
faces.

**Note:** Calibri is an Office font. LibreOffice will substitute (often Carlito or
similar) if Calibri isn’t installed. Built-in Arial remains the cross-platform
default when you omit `-template`.
