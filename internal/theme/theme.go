// Package theme holds shared layout and type settings for docx and PDF output.
package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Theme is a small set of knobs shared by the docx and PDF backends.
type Theme struct {
	MarginIn  float64   `json:"margin_in"`
	BodyPt    float64   `json:"body_pt"`
	CodePt    float64   `json:"code_pt"`
	HeadingPt []float64 `json:"heading_pt"`
	Font      string    `json:"font"`
	Mono      string    `json:"mono"`
}

// Default returns stencil's built-in theme (matches historical hard-coded values).
func Default() *Theme {
	return &Theme{
		MarginIn:  1.0,
		BodyPt:    12,
		CodePt:    10,
		HeadingPt: []float64{24, 18, 14, 12, 12, 12},
		Font:      "Arial",
		Mono:      "Courier New",
	}
}

// Load reads a JSON theme file and merges it over Default().
// Empty path returns Default().
func Load(path string) (*Theme, error) {
	t := Default()
	if path == "" {
		return t, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied -style flag
	if err != nil {
		return nil, fmt.Errorf("reading style %q: %w", path, err)
	}
	var overlay Theme
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, fmt.Errorf("parsing style %q: %w", path, err)
	}
	merge(t, &overlay)
	if err := t.Validate(); err != nil {
		return nil, fmt.Errorf("style %q: %w", path, err)
	}
	return t, nil
}

func merge(dst, src *Theme) {
	if src.MarginIn != 0 {
		dst.MarginIn = src.MarginIn
	}
	if src.BodyPt != 0 {
		dst.BodyPt = src.BodyPt
	}
	if src.CodePt != 0 {
		dst.CodePt = src.CodePt
	}
	if len(src.HeadingPt) > 0 {
		h := make([]float64, 6)
		copy(h, Default().HeadingPt)
		for i := 0; i < len(src.HeadingPt) && i < 6; i++ {
			if src.HeadingPt[i] != 0 {
				h[i] = src.HeadingPt[i]
			}
		}
		dst.HeadingPt = h
	}
	if strings.TrimSpace(src.Font) != "" {
		dst.Font = strings.TrimSpace(src.Font)
	}
	if strings.TrimSpace(src.Mono) != "" {
		dst.Mono = strings.TrimSpace(src.Mono)
	}
}

// Validate checks theme values are in a usable range.
func (t *Theme) Validate() error {
	if t.MarginIn < 0.25 || t.MarginIn > 2.0 {
		return fmt.Errorf("margin_in must be between 0.25 and 2.0, got %g", t.MarginIn)
	}
	if t.BodyPt <= 0 || t.BodyPt > 72 {
		return fmt.Errorf("body_pt must be in (0, 72], got %g", t.BodyPt)
	}
	if t.CodePt <= 0 || t.CodePt > 72 {
		return fmt.Errorf("code_pt must be in (0, 72], got %g", t.CodePt)
	}
	if len(t.HeadingPt) != 6 {
		return fmt.Errorf("heading_pt must have 6 entries, got %d", len(t.HeadingPt))
	}
	for i, pt := range t.HeadingPt {
		if pt <= 0 || pt > 96 {
			return fmt.Errorf("heading_pt[%d] must be in (0, 96], got %g", i, pt)
		}
	}
	if strings.TrimSpace(t.Font) == "" {
		return fmt.Errorf("font must be non-empty")
	}
	if strings.TrimSpace(t.Mono) == "" {
		return fmt.Errorf("mono must be non-empty")
	}
	return nil
}

// MarginTwips returns the margin in OOXML twips (1/20 pt, 1440 per inch).
func (t *Theme) MarginTwips() int {
	return int(t.MarginIn*1440 + 0.5)
}

// ContentWidthTwips is letter width minus left and right margins.
func (t *Theme) ContentWidthTwips() int {
	const letterW = 12240
	m := t.MarginTwips()
	return letterW - 2*m
}

// BodyHalfPoints is body size in OOXML half-points.
func (t *Theme) BodyHalfPoints() int { return int(t.BodyPt*2 + 0.5) }

// CodeHalfPoints is code size in OOXML half-points.
func (t *Theme) CodeHalfPoints() int { return int(t.CodePt*2 + 0.5) }

// HeadingHalfPoints returns heading level 1–6 size in half-points.
func (t *Theme) HeadingHalfPoints(level int) int {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	return int(t.HeadingPt[level-1]*2 + 0.5)
}

// HeadingPtAt returns heading level 1–6 size in points.
func (t *Theme) HeadingPtAt(level int) float64 {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	return t.HeadingPt[level-1]
}

// MarginPt returns margin in PDF points (72 pt/in).
func (t *Theme) MarginPt() float64 { return t.MarginIn * 72 }

// NormalizePDFFonts maps theme font names onto PDF built-in faces.
// PDF only ships Helvetica / Courier; unknown names fall back to those.
func NormalizePDFFonts(font, mono string) (bodyFace, monoFace string) {
	bodyFace = "Helvetica"
	monoFace = "Courier"
	switch strings.ToLower(strings.TrimSpace(font)) {
	case "helvetica", "arial", "sans", "sans-serif", "liberation sans", "calibri", "aptos",
		"times", "times new roman", "serif":
		bodyFace = "Helvetica"
	}
	switch strings.ToLower(strings.TrimSpace(mono)) {
	case "courier", "courier new", "consolas", "mono", "monospace", "menlo", "monaco":
		monoFace = "Courier"
	default:
		monoFace = "Courier"
	}
	return bodyFace, monoFace
}
