package repl

import (
	"reflect"
	"testing"

	"github.com/aisk/goblin/interpreter"
	"github.com/aisk/goblin/token"
)

func completionStrings(c *completer, line string) ([]string, int) {
	candidates, offset := c.Do([]rune(line), len([]rune(line)))
	result := make([]string, len(candidates))
	for i, candidate := range candidates {
		result[i] = string(candidate)
	}
	return result, offset
}

func TestREPLCompleter(t *testing.T) {
	s := interpreter.NewSession(".")
	if _, err := s.Eval(`type User(name) { func hello(self) { return self.name } }`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(`var user = User("alice")`); err != nil {
		t.Fatal(err)
	}
	c := &completer{session: s}

	tests := []struct {
		line       string
		candidates []string
		offset     int
	}{
		{line: "pri", candidates: []string{"nt"}, offset: 3},
		{line: "ret", candidates: []string{"urn"}, offset: 3},
		{line: "user.na", candidates: []string{"me"}, offset: 2},
		{line: "print(user.he", candidates: []string{"llo"}, offset: 2},
		{line: "user.name.trim_s", candidates: []string{"uffix"}, offset: 6},
		{line: "make().pu", candidates: []string{}, offset: 0},
		{line: "missing.na", candidates: []string{}, offset: 2},
	}

	for _, tt := range tests {
		got, offset := completionStrings(c, tt.line)
		if !reflect.DeepEqual(got, tt.candidates) || offset != tt.offset {
			t.Errorf("complete(%q) = (%v, %d), want (%v, %d)", tt.line, got, offset, tt.candidates, tt.offset)
		}
	}
}

func TestCompletionPathAtCursor(t *testing.T) {
	path, prefix, ok := completionPath([]rune("user.na + value"), len([]rune("user.na")))
	if !ok || !reflect.DeepEqual(path, []string{"user"}) || prefix != "na" {
		t.Fatalf("completionPath = (%v, %q, %v)", path, prefix, ok)
	}
}

// lexicalTokens are the grammar's alphabetic token names that are not
// keywords.
var lexicalTokens = map[string]bool{
	"INVALID": true, "newline": true, "comment": true, "empty": true, "id": true,
	"int_lit": true, "float_lit": true, "string_lit": true,
}

func TestKeywordsMatchGrammar(t *testing.T) {
	known := map[string]bool{}
	for _, kw := range keywords {
		if token.TokMap.Type(kw) == token.INVALID {
			t.Errorf("%q is completed as a keyword but the grammar has no such token", kw)
		}
		known[kw] = true
	}
	for typ := token.Type(0); ; typ++ {
		name := token.TokMap.Id(typ)
		if name == "unknown" {
			break
		}
		if isWord(name) && !known[name] && !lexicalTokens[name] {
			t.Errorf("grammar keyword %q is missing from the completion keywords", name)
		}
	}
}

func isWord(s string) bool {
	for _, r := range s {
		if !isIdentifierRune(r) {
			return false
		}
	}
	return s != ""
}

func TestBracketsBalanced(t *testing.T) {
	for src, want := range map[string]bool{
		"func f() {\n":       false,
		"func f() {}\n":      true,
		"print(\"(\")\n":     true,
		"var x = [1, # ]\n":  false,
		"print(\"\\\"{\")\n": true,
	} {
		if got := bracketsBalanced(src); got != want {
			t.Errorf("bracketsBalanced(%q) = %v, want %v", src, got, want)
		}
	}
}
