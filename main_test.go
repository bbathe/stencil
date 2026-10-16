package main

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		flags      []string
		positional []string
	}{
		{
			name:       "flags after input",
			args:       []string{"doc.md", "-output", "out.docx"},
			flags:      []string{"-output", "out.docx"},
			positional: []string{"doc.md"},
		},
		{
			name:       "stdin dash",
			args:       []string{"-", "-output", "out.docx"},
			flags:      []string{"-output", "out.docx"},
			positional: []string{"-"},
		},
		{
			name:       "equals form",
			args:       []string{"-output=out.docx", "doc.md"},
			flags:      []string{"-output=out.docx"},
			positional: []string{"doc.md"},
		},
		{
			name:       "version only",
			args:       []string{"-version"},
			flags:      []string{"-version"},
			positional: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, positional := splitArgs(tt.args)
			if !reflect.DeepEqual(flags, tt.flags) {
				t.Fatalf("flags: got %#v want %#v", flags, tt.flags)
			}
			if !reflect.DeepEqual(positional, tt.positional) {
				t.Fatalf("positional: got %#v want %#v", positional, tt.positional)
			}
		})
	}
}

func TestReplaceExt(t *testing.T) {
	got := replaceExt("reports/doc.md", ".docx")
	if got != "reports/doc.docx" {
		t.Fatalf("got %q", got)
	}
	got = replaceExt("noext", ".docx")
	if got != "noext.docx" {
		t.Fatalf("got %q", got)
	}
}
