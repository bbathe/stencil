// Package docx builds a minimal but complete .docx using the Go standard
// library (archive/zip + encoding/xml + image) plus optional WebP decode support.
//
// Supported OOXML parts:
//
//	[Content_Types].xml
//	_rels/.rels
//	docProps/core.xml
//	word/document.xml
//	word/styles.xml
//	word/settings.xml
//	word/numbering.xml
//	word/footnotes.xml      (when footnotes are present)
//	word/media/image*.ext   (when images are present)
//	word/_rels/document.xml.rels
//	word/_rels/footnotes.xml.rels (when footnotes reference links/images)
package docx

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/bbathe/stencil/internal/theme"
)

// fixedZipTime is used for all zip entry timestamps and core properties so
// identical content produces byte-identical .docx output.
var fixedZipTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Document accumulates content and serialises it to a .docx file.
type Document struct {
	body []bodyElement

	// relationships
	hyperlinks   []hyperlinkEntry
	hyperlinkMap map[string]string // url -> rId (dedup)
	images       []imageEntry

	// list numbering: numId 1 = bullets, 2..N = one per ordered list instance.
	// orderedListStarts[i] is the start value for numId i+2.
	orderedListStarts []int

	// drawingID is a monotonically increasing id for wp:docPr / pic:cNvPr.
	drawingID int

	// footnotes
	footnotes []footnoteEntry

	// BaseDir is prepended to relative local image paths (typically the
	// directory containing the source .md file). Empty means use the path as-is.
	BaseDir string

	// Theme controls margins, fonts, and type sizes. Nil means theme.Default().
	Theme *theme.Theme
}

type (
	hyperlinkEntry struct{ rId, url string }
	imageEntry     struct {
		rId, partName, mimeType, ext string
		data                         []byte
	}
)

// footnoteEntry holds the footnote id and a Para for its formatted content.
// The Para is written into during rendering and read back in footnotesXML.
type footnoteEntry struct {
	id   int
	para *Para
}

// New returns an empty Document.
func New() *Document { return &Document{} }

func (d *Document) themeOrDefault() *theme.Theme {
	if d.Theme != nil {
		return d.Theme
	}
	return theme.Default()
}

// ── Resource registration ─────────────────────────────────────────────────────

// RegisterHyperlink returns a stable rId for the given URL, creating one if needed.
func (d *Document) RegisterHyperlink(url string) string {
	if d.hyperlinkMap == nil {
		d.hyperlinkMap = make(map[string]string)
	}
	if rId, ok := d.hyperlinkMap[url]; ok {
		return rId
	}
	rId := fmt.Sprintf("rIdHL%d", len(d.hyperlinks)+1)
	d.hyperlinks = append(d.hyperlinks, hyperlinkEntry{rId: rId, url: url})
	d.hyperlinkMap[url] = rId
	return rId
}

// ResolveImagePath returns an absolute or cwd-relative filesystem path for a
// markdown image destination. Remote URLs and data URIs are returned unchanged
// (and will fail at open — callers should fall back to alt text).
func (d *Document) ResolveImagePath(dest string) string {
	if dest == "" {
		return dest
	}
	if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") ||
		strings.HasPrefix(dest, "data:") {
		return dest
	}
	// Strip optional file:// prefix.
	if strings.HasPrefix(dest, "file://") {
		if u, err := url.Parse(dest); err == nil {
			dest = u.Path
		}
	}
	if filepath.IsAbs(dest) || d.BaseDir == "" {
		return dest
	}
	return filepath.Join(d.BaseDir, filepath.FromSlash(dest))
}

// RegisterImage reads the image file at path, stores its bytes, and returns
// (rId, widthEMU, heightEMU, drawingID).  On error the image is skipped and
// ("", 0, 0, 0, err) is returned so the caller can fall back to alt-text.
// path should already be resolved (see ResolveImagePath).
func (d *Document) RegisterImage(path string) (string, int64, int64, int, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied image path
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("reading image %q: %w", path, err)
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	mime := "image/png"
	switch ext {
	case "jpg", "jpeg":
		mime = "image/jpeg"
		ext = "jpeg"
	case "gif":
		mime = "image/gif"
	case "webp":
		mime = "image/webp"
		ext = "webp"
	default:
		ext = "png"
	}

	n := len(d.images) + 1
	rId := fmt.Sprintf("rIdIM%d", n)
	partName := fmt.Sprintf("word/media/image%d.%s", n, ext)
	d.images = append(d.images, imageEntry{rId: rId, partName: partName, mimeType: mime, ext: ext, data: data})
	d.drawingID++
	drawingID := d.drawingID

	// Dimensions in EMU (1 inch = 914400, 96 DPI → 9525 EMU/pixel)
	const emuPerPx = int64(9525)
	maxW := int64(d.themeOrDefault().ContentWidthTwips()) * 635 // twip → EMU
	cfg, _, decErr := image.DecodeConfig(bytes.NewReader(data))
	if decErr != nil {
		return rId, 3657600, 2743200, drawingID, nil // 4" × 3" fallback
	}
	w := int64(cfg.Width) * emuPerPx
	h := int64(cfg.Height) * emuPerPx
	if w > maxW {
		h = h * maxW / w
		w = maxW
	}
	return rId, w, h, drawingID, nil
}

// AddFootnote registers a new footnote and returns its 1-based id and a *Para
// into which the caller writes the formatted footnote content (runs, hyperlinks, etc.).
func (d *Document) AddFootnote() (int, *Para) {
	id := len(d.footnotes) + 1
	p := &Para{}
	d.footnotes = append(d.footnotes, footnoteEntry{id: id, para: p})
	return id, p
}

// BulletNumID returns the fixed numId for unordered lists.
func (d *Document) BulletNumID() int { return 1 }

// NewOrderedNumID allocates a new numId for an ordered list instance.
// Each separate ordered list must get its own numId so numbering restarts.
// start is the 1-based initial number (markdown "3. item" → start=3); values
// below 1 are treated as 1.
func (d *Document) NewOrderedNumID(start int) int {
	if start < 1 {
		start = 1
	}
	d.orderedListStarts = append(d.orderedListStarts, start)
	return 1 + len(d.orderedListStarts) // 2, 3, 4, …
}
