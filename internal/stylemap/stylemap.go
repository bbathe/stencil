// Package stylemap loads and resolves the mapping from Markdown element types
// to Word style names. If no external file is provided, built-in defaults are used.
package stylemap

import (
	"encoding/json"
	"fmt"
	"os"
)

// StyleMap holds the mapping from element keys to Word style names.
type StyleMap struct {
	m map[string]string
}

// defaults is the built-in style map used when no external file is provided.
// Keys must match the constants defined in the renderer package.
var defaults = map[string]string{
	"heading1":   "Heading 1",
	"heading2":   "Heading 2",
	"heading3":   "Heading 3",
	"heading4":   "Heading 4",
	"heading5":   "Heading 5",
	"heading6":   "Heading 6",
	"body":       "Normal",
	"code_block": "Code",
	"blockquote": "Quote",
	"list_item":  "List Paragraph",
}

// Load returns a StyleMap. If path is empty, built-in defaults are used.
// If path is provided, the file is parsed as JSON and merged over the defaults
// so that any key not present in the file still falls back to its default value.
func Load(path string) (*StyleMap, error) {
	m := make(map[string]string, len(defaults))
	for k, v := range defaults {
		m[k] = v
	}

	if path == "" {
		return &StyleMap{m: m}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading style-map %q: %w", path, err)
	}

	var overrides map[string]string
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parsing style-map %q: %w", path, err)
	}

	for k, v := range overrides {
		m[k] = v
	}

	return &StyleMap{m: m}, nil
}

// Style returns the Word style name for the given element key.
// If the key is not found, "Normal" is returned as a safe fallback.
func (s *StyleMap) Style(key string) string {
	if v, ok := s.m[key]; ok {
		return v
	}
	return "Normal"
}
