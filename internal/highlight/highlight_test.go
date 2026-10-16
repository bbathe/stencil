package highlight

import (
	"strings"
	"testing"
)

func TestTokenizeUnknownLang(t *testing.T) {
	toks := Tokenize("hello", "nosuchlang")
	if len(toks) != 1 || toks[0].Text != "hello" || toks[0].Color != "" {
		t.Fatalf("unexpected: %#v", toks)
	}
}

func TestTokenizeGoKeywords(t *testing.T) {
	toks := Tokenize("func main()", "go")
	var joined strings.Builder
	sawKeyword := false
	for _, tok := range toks {
		joined.WriteString(tok.Text)
		if tok.Text == "func" && tok.Color == ColorKeyword {
			sawKeyword = true
		}
	}
	if joined.String() != "func main()" {
		t.Fatalf("round-trip: %q", joined.String())
	}
	if !sawKeyword {
		t.Fatal("expected func keyword colour")
	}
}
