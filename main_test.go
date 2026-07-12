package main

import (
	"reflect"
	"testing"

	"github.com/aisk/goblin/interpreter"
)

func completionStrings(c *replCompleter, line string) ([]string, int) {
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
	c := &replCompleter{session: s}

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
