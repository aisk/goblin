package regexp

import (
	"errors"
	"sync"
	"testing"

	"github.com/aisk/goblin/object"
)

func call(t *testing.T, receiver object.Object, name string, positional ...object.Object) object.Object {
	t.Helper()
	fn, err := receiver.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := object.Call(fn, object.CallArgs{Positional: positional})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func getAttr(t *testing.T, receiver object.Object, name string) object.Object {
	t.Helper()
	value, err := receiver.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func callErr(t *testing.T, receiver object.Object, name string, positional ...object.Object) error {
	t.Helper()
	fn, err := receiver.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	_, err = object.Call(fn, object.CallArgs{Positional: positional})
	if err == nil {
		t.Fatalf("%s() succeeded, want an error", name)
	}
	return err
}

func compilePattern(t *testing.T, source string) *Pattern {
	t.Helper()
	value, err := compile(object.CallArgs{Positional: object.Args{object.String(source)}})
	if err != nil {
		t.Fatal(err)
	}
	return value.(*Pattern)
}

func TestCompileValidAndInvalid(t *testing.T) {
	compilePattern(t, `[a-z]+`)
	_, err := compile(object.CallArgs{Positional: object.Args{object.String(`(`)}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("compile invalid error = %v, want ParseError", err)
	}
}

func TestEscapeQuotesMetacharacters(t *testing.T) {
	quoted, err := escape(object.CallArgs{Positional: object.Args{object.String(`a.b*c`)}})
	if err != nil {
		t.Fatal(err)
	}
	if quoted != object.String(`a\.b\*c`) {
		t.Fatalf("escape = %v", quoted)
	}
	p := compilePattern(t, string(quoted.(object.String)))
	if got := call(t, p, "match", object.String("a.b*c")); got != object.True {
		t.Fatalf("escaped pattern does not match its literal: %v", got)
	}
	if got := call(t, p, "match", object.String("axbbc")); got != object.False {
		t.Fatalf("escaped pattern matched a metacharacter expansion: %v", got)
	}
}

func TestFindSearchAndFull(t *testing.T) {
	p := compilePattern(t, `a|ab`)
	m := call(t, p, "find", object.String("zab")).(*Match)
	if m.substring(0) != "a" || m.indices[0] != 1 {
		t.Fatalf("find = %#v", m)
	}
	if got := call(t, p, "find", object.String("zab"), object.True); !got.Equals(object.Nil) {
		t.Fatalf("full find = %v, want nil", got)
	}
	if got := call(t, p, "find", object.String("ab"), object.True).(*Match).substring(0); got != "ab" {
		t.Fatalf("full alternation match = %q", got)
	}
	if got := call(t, p, "match", object.String("xx")); got != object.False {
		t.Fatalf("match = %v", got)
	}
	if got := call(t, p, "match", object.String("zab"), object.True); got != object.False {
		t.Fatalf("full match = %v", got)
	}
}

func TestPatternIntrospection(t *testing.T) {
	p := compilePattern(t, `(?P<word>[a-z]+)(\d+)?`)
	if got := getAttr(t, p, "pattern"); got != object.String(`(?P<word>[a-z]+)(\d+)?`) {
		t.Fatalf("pattern = %v", got)
	}
	names := getAttr(t, p, "group_names").(*object.List)
	if names.String() != `["word", nil]` {
		t.Fatalf("group_names = %v", names)
	}
	for _, attr := range p.Attributes() {
		if _, err := p.GetAttr(attr); err != nil {
			t.Fatalf("GetAttr(%q): %v", attr, err)
		}
	}
}

func TestMatchGroupsNamesAndByteOffsets(t *testing.T) {
	p := compilePattern(t, `(?P<word>[a-z]+)(?P<optional>\d+)?`)
	m := call(t, p, "find", object.String("日abc")).(*Match)
	if m.indices[0] != 3 || m.indices[1] != 6 {
		t.Fatalf("offsets = %v, want UTF-8 byte offsets 3..6", m.indices[:2])
	}
	if got := call(t, m, "group", object.Integer(1)); got != object.String("abc") {
		t.Fatalf("group(1) = %v", got)
	}
	if got := call(t, m, "group", object.String("word")); got != object.String("abc") {
		t.Fatalf("group(word) = %v", got)
	}
	if got := call(t, m, "group", object.String("optional")); !got.Equals(object.Nil) {
		t.Fatalf("optional group = %v", got)
	}
	if got := getAttr(t, m, "source"); got != object.String("日abc") {
		t.Fatalf("source = %v", got)
	}
	groups := m.groups()
	if len(groups.Elements) != 2 || !groups.Elements[1].Equals(object.Nil) {
		t.Fatalf("groups = %v", groups)
	}
	named := getAttr(t, m, "named_groups").(*object.Dict)
	namedString := named.String()
	if namedString != `{"word": "abc", "optional": nil}` && namedString != `{"optional": nil, "word": "abc"}` {
		t.Fatalf("named_groups = %v", named)
	}
	for _, attr := range m.Attributes() {
		if _, err := m.GetAttr(attr); err != nil {
			t.Fatalf("GetAttr(%q): %v", attr, err)
		}
	}
}

func TestMatchSpan(t *testing.T) {
	p := compilePattern(t, `(?P<word>[a-z]+)(\d+)?`)
	m := call(t, p, "find", object.String("12 abc")).(*Match)
	if got := call(t, m, "span").(*object.List); got.String() != "[3, 6]" {
		t.Fatalf("span() = %v", got)
	}
	if got := call(t, m, "span", object.String("word")).(*object.List); got.String() != "[3, 6]" {
		t.Fatalf("span(word) = %v", got)
	}
	if got := call(t, m, "span", object.Integer(2)); !got.Equals(object.Nil) {
		t.Fatalf("span of a non-participating group = %v, want nil", got)
	}
	if err := callErr(t, m, "span", object.Integer(9)); !errors.Is(err, object.IndexError) {
		t.Fatalf("span(9) error = %v, want IndexError", err)
	}
	if err := callErr(t, m, "group", object.True); !errors.Is(err, object.TypeError) {
		t.Fatalf("group(true) error = %v, want TypeError", err)
	}
}

func TestDuplicateNamedGroupUsesFirstParticipating(t *testing.T) {
	p := compilePattern(t, `(?P<x>a)|(?P<x>b)`)
	m := call(t, p, "find", object.String("b")).(*Match)
	if got := call(t, m, "group", object.String("x")); got != object.String("b") {
		t.Fatalf("group(x) = %v", got)
	}
	named := getAttr(t, m, "named_groups").(*object.Dict)
	if named.String() != `{"x": "b"}` {
		t.Fatalf("named_groups = %v", named)
	}
}

func TestFindAllCountAndZeroLength(t *testing.T) {
	p := compilePattern(t, `a*`)
	if got := call(t, p, "find_all", object.String("baa"), object.Integer(0)).(*object.List); len(got.Elements) != 0 {
		t.Fatalf("count 0 = %v", got)
	}
	if got := call(t, p, "find_all", object.String("baa"), object.Integer(1)).(*object.List); len(got.Elements) != 1 {
		t.Fatalf("count 1 = %v", got)
	}
	got := call(t, p, "find_all", object.String("baa")).(*object.List)
	if len(got.Elements) != 2 || got.Elements[0].(*Match).substring(0) != "" || got.Elements[1].(*Match).substring(0) != "aa" {
		t.Fatalf("zero-length matches = %v", got)
	}
}

func TestReplaceTemplateAndCount(t *testing.T) {
	p := compilePattern(t, `(?P<key>[a-z]+)=(\d+)`)
	got := call(t, p, "replace", object.String("a=1 b=2"), object.String(`${key}:$2`), object.Integer(1))
	if got != object.String("a:1 b=2") {
		t.Fatalf("replace = %v", got)
	}
	got = call(t, p, "replace", object.String("a=1"), object.String("x"), object.Integer(0))
	if got != object.String("a=1") {
		t.Fatalf("replace count 0 = %v", got)
	}
	// A malformed reference stays literal, matching Go's Expand.
	got = call(t, p, "replace", object.String("a=1"), object.String("$ $"))
	if got != object.String("$ $") {
		t.Fatalf("malformed template references = %v", got)
	}
	got = call(t, p, "replace", object.String("a=1"), object.String("$$1"))
	if got != object.String("$1") {
		t.Fatalf("escaped dollar = %v", got)
	}
}

func TestReplaceRejectsUnknownGroupReference(t *testing.T) {
	p := compilePattern(t, `(?P<key>[a-z]+)=(\d+)`)
	for _, template := range []string{"${missing}", "$missing", "$3", "${3}", "$1x"} {
		err := callErr(t, p, "replace", object.String("a=1"), object.String(template))
		if !errors.Is(err, object.ValueError) {
			t.Fatalf("template %q error = %v, want ValueError", template, err)
		}
	}
	// $0 is the whole match, and a valid reference.
	if got := call(t, p, "replace", object.String("a=1"), object.String("[$0]")); got != object.String("[a=1]") {
		t.Fatalf("replace with $0 = %v", got)
	}
}

func TestBytesAreRejected(t *testing.T) {
	p := compilePattern(t, `a`)
	err := callErr(t, p, "find", object.NewBytes([]byte{0xff}))
	if !errors.Is(err, object.TypeError) {
		t.Fatalf("error = %v, want TypeError", err)
	}
}

// TestSplitCountMatchesStrSplit pins split's count to the built-in
// str.split(sep, count) meaning: count is the number of pieces returned.
func TestSplitCountMatchesStrSplit(t *testing.T) {
	p := compilePattern(t, `,`)
	text := object.String("a,b,c")
	for _, count := range []int64{-2, -1, 0, 1, 2, 3, 4} {
		got := call(t, p, "split", text, object.Integer(count)).(*object.List)
		want := call(t, text, "split", object.String(","), object.Integer(count)).(*object.List)
		if got.String() != want.String() {
			t.Fatalf("split(count=%d) = %v, want str.split = %v", count, got, want)
		}
	}
}

// TestReplaceCountMatchesStrReplace pins replace's count to the built-in
// str.replace(old, new, count) meaning: count is the number of replacements.
func TestReplaceCountMatchesStrReplace(t *testing.T) {
	p := compilePattern(t, `a`)
	text := object.String("aaa")
	for _, count := range []int64{-2, -1, 0, 1, 2, 3, 4} {
		got := call(t, p, "replace", text, object.String("x"), object.Integer(count))
		want := call(t, text, "replace", object.String("a"), object.String("x"), object.Integer(count))
		if got != want {
			t.Fatalf("replace(count=%d) = %v, want str.replace = %v", count, got, want)
		}
	}
}

func TestPatternConcurrentReuse(t *testing.T) {
	p := compilePattern(t, `[a-z]+`)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// full=true races on the lazily compiled anchored engine.
				re, err := p.matcher(object.True)
				if err != nil {
					t.Error(err)
					return
				}
				if !re.MatchString("abc") || re.MatchString("a b") {
					t.Error("inconsistent concurrent full match")
					return
				}
				if !p.re.MatchString("abc") || p.re.MatchString("123") {
					t.Error("inconsistent concurrent match")
					return
				}
			}
		}()
	}
	wg.Wait()
}
