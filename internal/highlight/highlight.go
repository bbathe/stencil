// Package highlight provides a simple, allocation-light syntax tokenizer
// for use in DOCX code-block rendering.  It is intentionally small — no
// external dependencies, no regex, no full parser — just enough to give
// code blocks meaningful colour in a Word document.
package highlight

import "strings"

// ── Public API ────────────────────────────────────────────────────────────────

// Token is a contiguous span of source text with an optional colour.
type Token struct {
	Text  string // raw source text (may contain \n for block tokenisation)
	Color string // 6-digit hex RGB, empty = document default (black)
}

// Colour palette (print-friendly, similar to VS Code Light+)
const (
	ColorKeyword = "000080" // navy
	ColorString  = "A31515" // dark red
	ColorComment = "008000" // dark green
	ColorNumber  = "098658" // teal
	ColorType    = "267F99" // blue-green
	ColorBuiltin = "795E26" // brown / function gold
)

// Tokenize splits code into coloured tokens for the given language identifier
// (e.g. "go", "python", "javascript").  Unrecognised or empty lang returns a
// single unstyled token so the text is still emitted correctly.
func Tokenize(code, lang string) []Token {
	cfg, ok := languages[strings.ToLower(lang)]
	if !ok {
		if code == "" {
			return nil
		}
		return []Token{{Text: code}}
	}
	return tokenize(code, cfg)
}

// ── Language registry ─────────────────────────────────────────────────────────

type strDelim struct {
	start     string
	end       string
	escape    string // e.g. "\\" — advance past escape + next char
	multiline bool
}

type langConfig struct {
	keywords     map[string]bool
	types        map[string]bool
	builtins     map[string]bool
	lineComments []string  // e.g. ["//", "#"]
	blockComment [2]string // [open, close], both empty = none
	stringDelims []strDelim
}

func kwset(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

var languages = map[string]langConfig{
	"go": {
		keywords: kwset("break", "case", "chan", "const", "continue", "default",
			"defer", "else", "fallthrough", "for", "func", "go", "goto", "if",
			"import", "interface", "map", "package", "range", "return", "select",
			"struct", "switch", "type", "var"),
		types: kwset("bool", "byte", "complex64", "complex128", "error",
			"float32", "float64", "int", "int8", "int16", "int32", "int64",
			"rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr"),
		builtins: kwset("append", "cap", "close", "complex", "copy", "delete",
			"imag", "len", "make", "new", "panic", "print", "println", "real",
			"recover", "true", "false", "nil", "iota"),
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{"`", "`", "", true},
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"python": {
		keywords: kwset("False", "None", "True", "and", "as", "assert", "async",
			"await", "break", "class", "continue", "def", "del", "elif", "else",
			"except", "finally", "for", "from", "global", "if", "import", "in",
			"is", "lambda", "nonlocal", "not", "or", "pass", "raise", "return",
			"try", "while", "with", "yield"),
		types:        kwset("bool", "bytes", "bytearray", "complex", "dict", "float", "frozenset", "int", "list", "memoryview", "object", "range", "set", "str", "tuple", "type"),
		builtins:     kwset("abs", "all", "any", "bin", "callable", "chr", "compile", "delattr", "dir", "divmod", "enumerate", "eval", "exec", "filter", "format", "getattr", "globals", "hasattr", "hash", "help", "hex", "id", "input", "isinstance", "issubclass", "iter", "len", "locals", "map", "max", "min", "next", "oct", "open", "ord", "pow", "print", "property", "repr", "reversed", "round", "setattr", "slice", "sorted", "staticmethod", "sum", "super", "vars", "zip"),
		lineComments: []string{"#"},
		stringDelims: []strDelim{
			{`"""`, `"""`, "", true},
			{"'''", "'''", "", true},
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"javascript": goLikeJS(),
	"js":         goLikeJS(),
	"typescript": goLikeTS(),
	"ts":         goLikeTS(),
	"rust": {
		keywords: kwset("as", "async", "await", "break", "const", "continue",
			"crate", "dyn", "else", "enum", "extern", "fn", "for", "if", "impl",
			"in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref",
			"return", "self", "Self", "static", "struct", "super", "trait", "type",
			"unsafe", "use", "where", "while"),
		types: kwset("bool", "char", "f32", "f64", "i8", "i16", "i32", "i64",
			"i128", "isize", "str", "u8", "u16", "u32", "u64", "u128", "usize",
			"String", "Vec", "Option", "Result", "Box", "Rc", "Arc"),
		builtins:     kwset("true", "false", "None", "Some", "Ok", "Err", "println", "eprintln", "panic", "todo", "unimplemented", "unreachable"),
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"java": {
		keywords: kwset("abstract", "assert", "boolean", "break", "byte", "case",
			"catch", "char", "class", "const", "continue", "default", "do",
			"double", "else", "enum", "extends", "final", "finally", "float",
			"for", "goto", "if", "implements", "import", "instanceof", "int",
			"interface", "long", "native", "new", "package", "private", "protected",
			"public", "return", "short", "static", "strictfp", "super", "switch",
			"synchronized", "this", "throw", "throws", "transient", "try", "void",
			"volatile", "while"),
		types:        kwset("String", "Integer", "Long", "Double", "Float", "Boolean", "Byte", "Short", "Character", "Object", "List", "Map", "Set", "ArrayList", "HashMap"),
		builtins:     kwset("true", "false", "null"),
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"c":     cLang(),
	"cpp":   cppLang(),
	"c++":   cppLang(),
	"bash":  bashLang(),
	"sh":    bashLang(),
	"shell": bashLang(),
	"json": {
		// JSON has no keywords, just literals
		keywords:     kwset("true", "false", "null"),
		stringDelims: []strDelim{{`"`, `"`, `\`, false}},
	},
	"sql": {
		keywords: kwset(
			"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
			"ON", "AS", "AND", "OR", "NOT", "IN", "EXISTS", "BETWEEN", "LIKE",
			"ORDER", "BY", "GROUP", "HAVING", "LIMIT", "OFFSET", "INSERT", "INTO",
			"VALUES", "UPDATE", "SET", "DELETE", "CREATE", "TABLE", "DROP", "ALTER",
			"ADD", "COLUMN", "PRIMARY", "KEY", "FOREIGN", "REFERENCES", "INDEX",
			"UNIQUE", "NULL", "DEFAULT", "CONSTRAINT", "DATABASE", "USE", "SHOW",
			"DESCRIBE", "EXPLAIN", "BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION",
			// lowercase variants
			"select", "from", "where", "join", "left", "right", "inner", "outer",
			"on", "as", "and", "or", "not", "in", "exists", "between", "like",
			"order", "by", "group", "having", "limit", "offset", "insert", "into",
			"values", "update", "set", "delete", "create", "table", "drop", "alter",
			"add", "column", "primary", "key", "foreign", "references", "index",
			"unique", "null", "default", "constraint", "database", "use",
			"begin", "commit", "rollback", "transaction"),
		types:        kwset("INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "FLOAT", "DOUBLE", "DECIMAL", "NUMERIC", "VARCHAR", "CHAR", "TEXT", "BLOB", "DATE", "DATETIME", "TIMESTAMP", "BOOLEAN", "BOOL"),
		lineComments: []string{"--"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{"'", "'", `\`, false},
			{`"`, `"`, `\`, false},
		},
	},
	"yaml": {
		keywords:     kwset("true", "false", "null", "yes", "no", "on", "off"),
		lineComments: []string{"#"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", "", false},
		},
	},
	"yml": {
		keywords:     kwset("true", "false", "null", "yes", "no", "on", "off"),
		lineComments: []string{"#"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", "", false},
		},
	},
	"html": {
		lineComments: []string{},
		blockComment: [2]string{"<!--", "-->"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"css": {
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"kotlin": {
		keywords: kwset("as", "break", "class", "continue", "do", "else", "false",
			"for", "fun", "if", "in", "interface", "is", "null", "object", "package",
			"return", "super", "this", "throw", "true", "try", "typealias", "typeof",
			"val", "var", "when", "while", "by", "catch", "constructor", "delegate",
			"dynamic", "field", "file", "finally", "get", "import", "init", "param",
			"property", "receiver", "set", "setparam", "value", "where",
			"actual", "abstract", "annotation", "companion", "crossinline",
			"data", "enum", "expect", "external", "final", "infix", "inline",
			"inner", "internal", "lateinit", "noinline", "open", "operator",
			"out", "override", "private", "protected", "public", "reified",
			"sealed", "suspend", "tailrec", "vararg"),
		types:        kwset("Boolean", "Byte", "Char", "Double", "Float", "Int", "Long", "Short", "String", "Unit", "Any", "Nothing", "Array", "List", "Map", "Set", "Pair", "Triple"),
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{`"""`, `"""`, "", true},
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
	"ruby": {
		keywords: kwset("__ENCODING__", "__LINE__", "__FILE__", "BEGIN", "END",
			"alias", "and", "begin", "break", "case", "class", "def", "defined",
			"do", "else", "elsif", "end", "ensure", "false", "for", "if", "in",
			"module", "next", "nil", "not", "or", "redo", "rescue", "retry",
			"return", "self", "super", "then", "true", "undef", "unless", "until",
			"when", "while", "yield"),
		lineComments: []string{"#"},
		blockComment: [2]string{"=begin", "=end"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	},
}

func goLikeJS() langConfig {
	return langConfig{
		keywords: kwset("break", "case", "catch", "class", "const", "continue",
			"debugger", "default", "delete", "do", "else", "export", "extends",
			"finally", "for", "function", "if", "import", "in", "instanceof",
			"let", "new", "return", "static", "super", "switch", "this", "throw",
			"try", "typeof", "var", "void", "while", "with", "yield", "async", "await", "of"),
		types:        kwset("Array", "Boolean", "Date", "Error", "Function", "Map", "Number", "Object", "Promise", "RegExp", "Set", "String", "Symbol", "WeakMap", "WeakSet", "undefined"),
		builtins:     kwset("true", "false", "null", "NaN", "Infinity", "console", "Math", "JSON", "window", "document", "process", "require", "module", "exports", "setTimeout", "setInterval", "clearTimeout", "clearInterval"),
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{"`", "`", `\`, true},
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	}
}

func goLikeTS() langConfig {
	cfg := goLikeJS()
	cfg.keywords["interface"] = true
	cfg.keywords["type"] = true
	cfg.keywords["enum"] = true
	cfg.keywords["namespace"] = true
	cfg.keywords["abstract"] = true
	cfg.keywords["implements"] = true
	cfg.keywords["readonly"] = true
	cfg.keywords["as"] = true
	cfg.keywords["from"] = true
	cfg.keywords["declare"] = true
	cfg.keywords["override"] = true
	cfg.types["never"] = true
	cfg.types["any"] = true
	cfg.types["unknown"] = true
	cfg.types["void"] = true
	cfg.types["string"] = true
	cfg.types["number"] = true
	cfg.types["boolean"] = true
	cfg.types["bigint"] = true
	cfg.types["symbol"] = true
	cfg.types["object"] = true
	return cfg
}

func cLang() langConfig {
	return langConfig{
		keywords: kwset("auto", "break", "case", "char", "const", "continue",
			"default", "do", "double", "else", "enum", "extern", "float", "for",
			"goto", "if", "inline", "int", "long", "register", "restrict",
			"return", "short", "signed", "sizeof", "static", "struct", "switch",
			"typedef", "union", "unsigned", "void", "volatile", "while"),
		builtins:     kwset("NULL", "true", "false", "printf", "scanf", "malloc", "calloc", "realloc", "free", "memcpy", "memset", "strlen", "strcpy", "strcmp"),
		lineComments: []string{"//"},
		blockComment: [2]string{"/*", "*/"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", `\`, false},
		},
	}
}

func cppLang() langConfig {
	cfg := cLang()
	for _, kw := range []string{
		"alignas", "alignof", "and", "and_eq", "asm",
		"bitand", "bitor", "bool", "catch", "class", "compl", "concept",
		"consteval", "constexpr", "constinit", "const_cast", "co_await",
		"co_return", "co_yield", "decltype", "delete", "dynamic_cast", "explicit",
		"export", "false", "friend", "mutable", "namespace", "new", "noexcept",
		"not", "not_eq", "nullptr", "operator", "or", "or_eq", "private",
		"protected", "public", "reinterpret_cast", "requires", "static_assert",
		"static_cast", "template", "this", "thread_local", "throw", "true", "try",
		"typeid", "typename", "using", "virtual", "wchar_t", "xor", "xor_eq",
	} {
		cfg.keywords[kw] = true
	}
	return cfg
}

func bashLang() langConfig {
	return langConfig{
		keywords:     kwset("if", "then", "else", "elif", "fi", "for", "while", "do", "done", "case", "esac", "in", "function", "return", "local", "export", "readonly", "declare", "unset", "shift", "source", "break", "continue"),
		builtins:     kwset("echo", "printf", "read", "cd", "pwd", "ls", "mkdir", "rm", "cp", "mv", "cat", "grep", "sed", "awk", "find", "chmod", "chown", "exit", "test", "true", "false", "set", "exec"),
		lineComments: []string{"#"},
		stringDelims: []strDelim{
			{`"`, `"`, `\`, false},
			{"'", "'", "", false},
		},
	}
}

// ── Core tokenizer ────────────────────────────────────────────────────────────

func tokenize(code string, cfg langConfig) []Token {
	var out []Token
	i := 0
	n := len(code)

	for i < n {
		// ── Line comment ─────────────────────────────────────────────────────
		if adv := matchLineComment(code, i, cfg.lineComments); adv > 0 {
			out = append(out, Token{Text: code[i : i+adv], Color: ColorComment})
			i += adv
			continue
		}

		// ── Block comment ────────────────────────────────────────────────────
		open, close := cfg.blockComment[0], cfg.blockComment[1]
		if open != "" && strings.HasPrefix(code[i:], open) {
			rest := code[i+len(open):]
			end := strings.Index(rest, close)
			var text string
			if end < 0 {
				text = code[i:]
				i = n
			} else {
				end = i + len(open) + end + len(close)
				text = code[i:end]
				i = end
			}
			out = append(out, Token{Text: text, Color: ColorComment})
			continue
		}

		// ── String literal ───────────────────────────────────────────────────
		if tok, adv := matchString(code, i, cfg.stringDelims); adv > 0 {
			out = append(out, Token{Text: tok, Color: ColorString})
			i += adv
			continue
		}

		// ── Number ───────────────────────────────────────────────────────────
		if adv := matchNumber(code, i); adv > 0 {
			out = append(out, Token{Text: code[i : i+adv], Color: ColorNumber})
			i += adv
			continue
		}

		// ── Identifier / keyword ─────────────────────────────────────────────
		if adv := matchIdent(code, i); adv > 0 {
			word := code[i : i+adv]
			color := ""
			if cfg.keywords[word] {
				color = ColorKeyword
			} else if cfg.types[word] {
				color = ColorType
			} else if cfg.builtins[word] {
				color = ColorBuiltin
			}
			out = append(out, Token{Text: word, Color: color})
			i += adv
			continue
		}

		// ── Default: single character ────────────────────────────────────────
		out = append(out, Token{Text: string(code[i])})
		i++
	}

	return out
}

// ── Match helpers ─────────────────────────────────────────────────────────────

func matchLineComment(code string, pos int, prefixes []string) int {
	for _, p := range prefixes {
		if strings.HasPrefix(code[pos:], p) {
			nl := strings.IndexByte(code[pos:], '\n')
			if nl < 0 {
				return len(code) - pos
			}
			return nl // exclude the newline itself
		}
	}
	return 0
}

func matchString(code string, pos int, delims []strDelim) (string, int) {
	for _, d := range delims {
		if !strings.HasPrefix(code[pos:], d.start) {
			continue
		}
		i := pos + len(d.start)
		for i < len(code) {
			if d.escape != "" && strings.HasPrefix(code[i:], d.escape) && i+len(d.escape) < len(code) {
				i += len(d.escape) + 1
				continue
			}
			if strings.HasPrefix(code[i:], d.end) {
				i += len(d.end)
				return code[pos:i], i - pos
			}
			if !d.multiline && code[i] == '\n' {
				// unterminated single-line string — emit what we have
				return code[pos:i], i - pos
			}
			i++
		}
		return code[pos:i], i - pos
	}
	return "", 0
}

func matchNumber(code string, pos int) int {
	if pos >= len(code) {
		return 0
	}
	// Hex: 0x / 0X
	if pos+2 <= len(code) && code[pos] == '0' && (code[pos+1] == 'x' || code[pos+1] == 'X') {
		i := pos + 2
		for i < len(code) && isHex(code[i]) {
			i++
		}
		if i > pos+2 {
			return i - pos
		}
	}
	if !isDigit(code[pos]) {
		return 0
	}
	i := pos
	for i < len(code) && isDigit(code[i]) {
		i++
	}
	if i < len(code) && code[i] == '.' && i+1 < len(code) && isDigit(code[i+1]) {
		i++
		for i < len(code) && isDigit(code[i]) {
			i++
		}
	}
	if i < len(code) && (code[i] == 'e' || code[i] == 'E') {
		j := i + 1
		if j < len(code) && (code[j] == '+' || code[j] == '-') {
			j++
		}
		if j < len(code) && isDigit(code[j]) {
			i = j
			for i < len(code) && isDigit(code[i]) {
				i++
			}
		}
	}
	return i - pos
}

func matchIdent(code string, pos int) int {
	if pos >= len(code) {
		return 0
	}
	c := code[pos]
	if !isLetter(c) && c != '_' {
		return 0
	}
	i := pos + 1
	for i < len(code) && (isLetter(code[i]) || isDigit(code[i]) || code[i] == '_') {
		i++
	}
	return i - pos
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isHex(c byte) bool    { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
