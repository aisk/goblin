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
	if got := call(t, p, "test", object.String("xx")); got != object.False {
		t.Fatalf("test = %v", got)
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
	groups := m.groups()
	if len(groups.Elements) != 2 || !groups.Elements[1].Equals(object.Nil) {
		t.Fatalf("groups = %v", groups)
	}
	for _, attr := range m.Attributes() {
		if _, err := m.GetAttr(attr); err != nil {
			t.Fatalf("GetAttr(%q): %v", attr, err)
		}
	}
}

func TestDuplicateNamedGroupUsesFirstParticipating(t *testing.T) {
	p := compilePattern(t, `(?P<x>a)|(?P<x>b)`)
	m := call(t, p, "find", object.String("b")).(*Match)
	if got := call(t, m, "group", object.String("x")); got != object.String("b") {
		t.Fatalf("group(x) = %v", got)
	}
}

func TestFindAllLimitsAndZeroLength(t *testing.T) {
	p := compilePattern(t, `a*`)
	if got := call(t, p, "find_all", object.String("baa"), object.Integer(0)).(*object.List); len(got.Elements) != 0 {
		t.Fatalf("limit 0 = %v", got)
	}
	if got := call(t, p, "find_all", object.String("baa"), object.Integer(1)).(*object.List); len(got.Elements) != 1 {
		t.Fatalf("limit 1 = %v", got)
	}
	got := call(t, p, "find_all", object.String("baa")).(*object.List)
	if len(got.Elements) != 2 || got.Elements[0].(*Match).substring(0) != "" || got.Elements[1].(*Match).substring(0) != "aa" {
		t.Fatalf("zero-length matches = %v", got)
	}
}

func TestReplaceTemplateAndLimit(t *testing.T) {
	p := compilePattern(t, `(?P<key>[a-z]+)=(\d+)`)
	got := call(t, p, "replace", object.String("a=1 b=2"), object.String(`${key}:$2`), object.Integer(1))
	if got != object.String("a:1 b=2") {
		t.Fatalf("replace = %v", got)
	}
	got = call(t, p, "replace", object.String("a=1"), object.String("x"), object.Integer(0))
	if got != object.String("a=1") {
		t.Fatalf("replace limit 0 = %v", got)
	}
	got = call(t, p, "replace", object.String("a=1"), object.String("$ ${missing} $missing"))
	if got != object.String("$  ") {
		t.Fatalf("malformed and unknown template references = %v", got)
	}
}

func TestBytesAreRejected(t *testing.T) {
	p := compilePattern(t, `a`)
	fn, _ := p.GetAttr("find")
	_, err := object.Call(fn, object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0xff})}})
	if err == nil || !errors.Is(err, object.TypeError) {
		t.Fatalf("error = %v, want TypeError", err)
	}
}

func TestSplitLimitCountsSeparators(t *testing.T) {
	p := compilePattern(t, `,\s*`)
	got := call(t, p, "split", object.String("a, b,c"), object.Integer(1)).(*object.List)
	if got.String() != `["a", "b,c"]` {
		t.Fatalf("split = %v", got)
	}
	got = call(t, p, "split", object.String("a,b"), object.Integer(0)).(*object.List)
	if got.String() != `["a,b"]` {
		t.Fatalf("split limit 0 = %v", got)
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
				if !p.re.MatchString("abc") || p.re.MatchString("123") {
					t.Error("inconsistent concurrent match")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestNegativeLimitRejected(t *testing.T) {
	p := compilePattern(t, `a`)
	fn, _ := p.GetAttr("find_all")
	_, err := object.Call(fn, object.CallArgs{Positional: object.Args{object.String("a"), object.Integer(-2)}})
	if err == nil || !errors.Is(err, object.ValueError) {
		t.Fatalf("error = %v, want ValueError", err)
	}
}
