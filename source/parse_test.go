package source

import (
	"os"
	"strings"
	"testing"

	"github.com/aisk/goblin/ast"
	"github.com/aisk/goblin/object"
	"github.com/aisk/goblin/semantic"
)

// Newlines terminate statements, so a line can only continue the previous
// one after a binary operator, a comma, a colon, or inside open brackets.
func TestParseLineStructure(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // substring of the error, or "" when the source must parse
	}{
		// A line-leading unary sign starts a new statement instead of continuing
		// the previous line; the semantic checker then rejects the unused value.
		{"minus after literal is a new statement", "var a = 10\n- 2\nprint(a)\n", ""},
		{"return then negative literal", "func f() {\n    return\n    -1\n}\n", ""},
		{"star after identifier", "var c = a\n* 2\n", `2:1: error: `},
		{"and after call", "foo()\n&& bar()\n", `2:1: error: `},
		{"return then call on next line", "func f() {\n    return\n    g()\n}\n", ""},
		{"two statements on one line", "var a = 1 var b = 2\n", `1:11: error: `},
		{"operator at end of line", "var a = 10 -\n    2\n", ""},
		{"logical operator at end of line", "var ok = a &&\n    b ||\n    c\n", ""},
		{"multi-line call", "print(1,\n    2,\n    3)\n", ""},
		{"multi-line call with newline after paren", "print(\n    1,\n    2\n)\n", ""},
		{"multi-line list", "var l = [\n    1,\n    2\n]\n", ""},
		{"multi-line dict", "var d = {\n    \"a\": 1,\n    \"b\":\n        2\n}\n", ""},
		{"multi-line parameters", "func f(\n    a,\n    b = 2\n) {\n    return a + b\n}\n", ""},
		{"multi-line type fields", "type T(\n    a,\n    b\n) {\n    func m(self) {}\n\n    func n(self) {}\n}\n", ""},
		{"parenthesised expression across lines", "var a = (\n    10 -\n    2\n)\n", ""},
		{"bare expression statement", "var a = 1\nvar b = 2\na + b\n", ""},
		{"statement starting with paren", "var a = 1\n(a).len()\n", ""},
		{"statement starting with bracket", "[1, 2].len()\n", ""},
		{"assignment to identifier", "var a = 1\na = 2\n", ""},
		{"keyword argument", "f(a = 1)\n", ""},
		{"comments as terminators", "var a = 1 # one\n# alone\nvar b = 2 # two", ""},
		{"comment inside multi-line call", "print(1, # first\n    2)\n", ""},
		{"comment after opening brace", "if true { # start\n    print(1)\n} # end\n", ""},
		{"blank lines and leading newlines", "\n\n\nvar a = 1\n\n\n", ""},
		{"empty source", "", ""},
		{"comment only", "# nothing\n", ""},
		{"single-line block", "if true { print(1) }\n", ""},
		{"crlf line endings", "var a = 1\r\nprint(a) # c\r\n", ""},
		{"else on its own line", "if true {\n}\nelse {\n}\n", `3:1: error: `},
		{"same line expression", "var a = 10 - 2\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(tt.src)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestParseReportsFilename(t *testing.T) {
	path := t.TempDir() + "/glue.goblin"
	if err := os.WriteFile(path, []byte("var a = 1 var b = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil || !strings.HasPrefix(err.Error(), path+":1:11: error: ") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Number literals: prefixed bases, '_' separators between digits, float
// exponents, and a decimal integer with a leading zero is an error.
func TestParseNumberLiterals(t *testing.T) {
	values := map[string]string{
		"0x1F": "31", "0xff_FF": "65535", "0o644": "420", "0b1010": "10",
		"1_000_000": "1000000", "0": "0", "9223372036854775807": "9223372036854775807",
		"1e9": "1000000000.0", "1.5e-3": "0.0015", "2E+2": "200.0", "1_000.25": "1000.25",
	}
	for src, want := range values {
		mod, err := ParseString("var x = " + src + "\n")
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		got, _ := mod.Body[0].(*ast.Declare).Value.(*ast.Literal).Value.ToString()
		if got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	errs := map[string]string{
		"0644":                "write 0o644 for an octal number",
		"1__0":                "'_' must separate two digits",
		"1_":                  "'_' must separate two digits",
		"0x_F":                "'_' must separate two digits",
		"1_.5":                "'_' must separate two digits",
		"0o8":                 "invalid digit in octal literal 0o8",
		"0b102":               "invalid digit in binary literal 0b102",
		"9223372036854775808": "out of range for Int",
		"1e400":               "out of range for Float",
	}
	for src, want := range errs {
		_, err := ParseString("var x = " + src + "\n")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want one containing %q", src, err, want)
		}
	}
}

// String literals: an interpolated expression is parsed in place, with
// positions inside the literal, and malformed literals are errors at the
// offending character.
func TestParseStringLiterals(t *testing.T) {
	mod, err := ParseString("var s = \"a{b}c{d.e(1)}\"\n")
	if err != nil {
		t.Fatal(err)
	}
	interp, ok := mod.Body[0].(*ast.Declare).Value.(*ast.Interpolation)
	if !ok || len(interp.Parts) != 4 {
		t.Fatalf("value = %#v, want an Interpolation of 4 parts", mod.Body[0].(*ast.Declare).Value)
	}
	if id, ok := interp.Parts[1].(*ast.Identifier); !ok || id.Name != "b" || id.Position().Line != 1 || id.Position().Column != 12 {
		t.Errorf("part 1 = %#v, want identifier b at 1:12", interp.Parts[1])
	}
	if _, ok := interp.Parts[3].(*ast.CallExpression); !ok {
		t.Errorf("part 3 = %#v, want a call", interp.Parts[3])
	}
	if lit, ok := mod.Body[0].(*ast.Declare).Value.(*ast.Interpolation).Parts[0].(*ast.Literal); !ok || lit.Value != object.String("a") {
		t.Errorf("part 0 = %#v, want \"a\"", interp.Parts[0])
	}

	plain, err := ParseString("var s = \"\\{x\\} \\x41\\u{1F600}\\0\"\nvar r = `a\\n{b}\r\nc`\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := plain.Body[0].(*ast.Declare).Value.(*ast.Literal).Value; got != object.String("{x} A\U0001F600\x00") {
		t.Errorf("escaped literal = %q", got)
	}
	if got := plain.Body[1].(*ast.Declare).Value.(*ast.Literal).Value; got != object.String("a\\n{b}\nc") {
		t.Errorf("raw literal = %q", got)
	}

	errs := []struct{ src, want string }{
		{`var s = "ab{}"`, "1:12: error: empty interpolation; write \\{ for a literal brace"},
		{`var s = "{  }"`, "1:10: error: empty interpolation"},
		{`var s = "{x:5}"`, "1:10: error: format specifiers are not supported"},
		{`var s = "{x[1:2]}"`, ""},
		{`var s = "{x"`, "1:10: error: unterminated interpolation"},
		{"var s = \"{x\n}\"", "1:10: error: an interpolation cannot span lines"},
		{`var s = "{x\"}"`, "1:10: error: an interpolation cannot contain a backslash"},
		{`var s = "a\u{110000}"`, "1:11: error: invalid Unicode escape \\u{110000}"},
		{`var s = "\u{D800}"`, "invalid Unicode escape \\u{D800}"},
		{`var s = "\u{0000041}"`, "invalid Unicode escape \\u{0000041}"},
		{`var s = "\q"`, "1:9: error: "},
		{`var s = "\x4"`, "1:9: error: "},
		{`var s = "{1 +}"`, "1:14: error: "},
		{`var s = "{a = 1}"`, "1:11: error: an interpolation must be a single expression"},
		{"var t = 1\n\nvar s = \"x\" + \"{b}\" + \"{1 +}\"\n", "3:28: error: "},
	}
	for _, tt := range errs {
		_, err := ParseString(tt.src + "\n")
		if tt.want == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", tt.src, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error %v, want one containing %q", tt.src, err, tt.want)
		}
	}
}

// An interpolated expression is checked like any other: an undefined name
// is reported where it stands inside the literal.
func TestCheckInterpolatedExpression(t *testing.T) {
	mod, err := ParseString("var a = 1\nprint(\"{a} {missing}\")\n")
	if err != nil {
		t.Fatal(err)
	}
	err = semantic.CheckModule(mod)
	if err == nil || !strings.Contains(err.Error(), "2:13: semantic error: undefined identifier: missing") {
		t.Fatalf("CheckModule error = %v", err)
	}
}
