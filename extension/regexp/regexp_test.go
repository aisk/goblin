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

func compileRegexp(t *testing.T, source string) *Regexp {
	t.Helper()
	value, err := compile(object.CallArgs{Positional: object.Args{object.String(source)}})
	if err != nil {
		t.Fatal(err)
	}
	return value.(*Regexp)
}

func TestModuleFunctions(t *testing.T) {
	if got, err := matchString(object.CallArgs{Positional: object.Args{object.String("a+"), object.String("baa")}}); err != nil || got != object.True {
		t.Fatalf("match_string = %v, %v", got, err)
	}
	if got, err := quoteMeta(object.CallArgs{Positional: object.Args{object.String("a+b")}}); err != nil || got != object.String(`a\+b`) {
		t.Fatalf("quote_meta = %v, %v", got, err)
	}
	_, err := compile(object.CallArgs{Positional: object.Args{object.String(`(`)}})
	if err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("compile invalid error = %v, want ParseError", err)
	}
}

func TestFindStringOptionsMirrorGoFamily(t *testing.T) {
	r := compileRegexp(t, `(?P<word>[a-z]+)(\d+)?`)
	if got := call(t, r, "find_string", object.String("日abc12")); got != object.String("abc12") {
		t.Fatalf("find_string = %v", got)
	}
	fn, _ := r.GetAttr("find_string")
	got, err := object.Call(fn, object.CallArgs{
		Positional: object.Args{object.String("日abc12 xyz")},
		Keyword:    map[string]object.Object{"all": object.True, "submatch": object.True, "index": object.True},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "[[3, 8, 3, 6, 6, 8], [9, 12, 9, 12, -1, -1]]" {
		t.Fatalf("combined find = %v", got)
	}
	none := call(t, r, "find_string", object.String("123"))
	if none != object.String("") {
		t.Fatalf("no match = %v", none)
	}
}

func TestReplaceSplitNamesAndString(t *testing.T) {
	r := compileRegexp(t, `(?P<key>[a-z]+)=(\d+)`)
	if got := call(t, r, "replace_all_string", object.String("a=1 b=2"), object.String("${key}:$2")); got != object.String("a:1 b:2") {
		t.Fatalf("replace = %v", got)
	}
	if got := call(t, compileRegexp(t, `,\s*`), "split", object.String("a, b,c"), object.Integer(2)); got.String() != `["a", "b,c"]` {
		t.Fatalf("split = %v", got)
	}
	if got := call(t, r, "subexp_names"); got.String() != `["", "key", ""]` {
		t.Fatalf("subexp_names = %v", got)
	}
	text, err := r.ToString()
	if err != nil || text != r.source {
		t.Fatalf("ToString = %q, %v", text, err)
	}
}

func TestRegexpConcurrentReuse(t *testing.T) {
	r := compileRegexp(t, `[a-z]+`)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if !r.re.MatchString("abc") || r.re.MatchString("123") {
					t.Error("inconsistent concurrent match")
					return
				}
			}
		}()
	}
	wg.Wait()
}
