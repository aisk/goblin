package source

import (
	"os"
	"strings"
	"testing"
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
			_, err := Parse(NewLexer([]byte(tt.src)))
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
	l, err := NewLexerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Parse(l)
	if err == nil || !strings.HasPrefix(err.Error(), path+":1:11: error: ") {
		t.Fatalf("unexpected error: %v", err)
	}
}
