package regex

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/aisk/goblin/object"
)

func callFn(name string, args object.Args, kws ...object.KwArg) (object.Object, error) {
	m, _ := Execute()
	return m.(*object.Module).Members[name].(*object.Function).Call(object.CallArgs{Positional: args, Keyword: kws})
}

func mustCall(t *testing.T, name string, args object.Args, kws ...object.KwArg) object.Object {
	t.Helper()
	v, err := callFn(name, args, kws...)
	if err != nil {
		t.Fatalf("%s(%v) error = %v", name, args, err)
	}
	return v
}

func s(text string) object.String { return object.String(text) }

func attr(t *testing.T, v object.Object, name string) object.Object {
	t.Helper()
	got, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func method(v object.Object, name string, args ...object.Object) (object.Object, error) {
	fn, err := v.GetAttr(name)
	if err != nil {
		return nil, err
	}
	return object.Call(fn, object.CallArgs{Positional: args})
}

func show(v object.Object) string {
	if st, ok := v.(fmt.Stringer); ok {
		return st.String()
	}
	text, _ := v.ToString()
	return text
}

func TestFind(t *testing.T) {
	m := mustCall(t, "find", object.Args{s(`(\w+)@(\w+)\.com`), s("mail ada@example.com today")})
	if attr(t, m, "text") != s("ada@example.com") || attr(t, m, "start") != object.Integer(5) || attr(t, m, "end") != object.Integer(20) {
		t.Fatalf("match = %s", show(m))
	}
	if show(attr(t, m, "groups")) != `["ada", "example"]` {
		t.Fatalf("groups = %s", show(attr(t, m, "groups")))
	}
	// groups is a new List on every read, so changing it leaves the Match
	// as it was.
	groups := attr(t, m, "groups").(*object.List)
	if groups == attr(t, m, "groups") {
		t.Fatal("groups is the same List on every read")
	}
	groups.Elements = append(groups.Elements, s("changed"))
	if show(attr(t, m, "groups")) != `["ada", "example"]` {
		t.Fatalf("changing groups changed the Match: %s", show(attr(t, m, "groups")))
	}
	if show(m) != `<Match "ada@example.com">` {
		t.Fatalf("display = %s", show(m))
	}
	if m.(*Match).Constructor() != MatchType.Function {
		t.Fatal("type_of(Match) is not regex.Match")
	}
	if truthy, _ := m.ToBool(); !truthy {
		t.Fatal("a Match is truthy")
	}
	if got := mustCall(t, "find", object.Args{s(`x`), s("abc")}); got != object.Nil {
		t.Fatalf("miss = %v", got)
	}
	// full=true must match the whole text.
	if got := mustCall(t, "find", object.Args{s(`\d+`), s("12a")}, object.KwArg{Name: "full", Value: object.True}); got != object.Nil {
		t.Fatalf("full miss = %v", got)
	}
	full := mustCall(t, "find", object.Args{s(`a|ab`), s("ab")}, object.KwArg{Name: "full", Value: object.True})
	if attr(t, full, "text") != s("ab") {
		t.Fatalf("full alternation = %v", attr(t, full, "text"))
	}
}

func TestCharacterPositions(t *testing.T) {
	text := "héllo wörld ünïcode"
	m := mustCall(t, "find", object.Args{s(`w(?P<mid>ö)rld`), s(text)})
	start, end := attr(t, m, "start").(object.Integer), attr(t, m, "end").(object.Integer)
	if start != 6 || end != 11 {
		t.Fatalf("start, end = %d, %d", start, end)
	}
	runes := []rune(text)
	if string(runes[start:end]) != "wörld" {
		t.Fatal("positions do not slice the text")
	}
	span, err := method(m, "span", s("mid"))
	if err != nil || show(span) != "[7, 8]" {
		t.Fatalf("span = %v, %v", span, err)
	}
	all := mustCall(t, "find_all", object.Args{s(`ü|ö`), s(text)}).(*object.List)
	if attr(t, all.Elements[1], "start") != object.Integer(12) {
		t.Fatalf("second start = %v", attr(t, all.Elements[1], "start"))
	}
}

func TestIndexSpanToDict(t *testing.T) {
	m := mustCall(t, "find", object.Args{s(`(?P<key>\w+)=(?P<value>\d+)?(x)?`), s("port=")})
	cases := map[object.Object]object.Object{
		object.Integer(0): s("port="), object.Integer(1): s("port"), object.Integer(2): object.Nil,
		s("key"): s("port"), s("value"): object.Nil, object.Integer(3): object.Nil,
	}
	for key, want := range cases {
		got, err := m.Index(key)
		if err != nil || got != want {
			t.Errorf("m[%v] = %v, %v", key, got, err)
		}
	}
	for _, key := range []object.Object{object.Integer(4), object.Integer(-1), s("nope")} {
		_, err := m.Index(key)
		if !errors.Is(err, object.IndexError) || err.Error() != "no such capture group: "+fmt.Sprint(key) {
			t.Errorf("m[%v] = %v", key, err)
		}
		if _, err := method(m, "span", key); !errors.Is(err, object.IndexError) {
			t.Errorf("span(%v) = %v", key, err)
		}
	}
	if _, err := m.Index(object.Float(1)); !errors.Is(err, object.TypeError) {
		t.Errorf("m[1.0] = %v", err)
	}
	if span, _ := method(m, "span", s("value")); span != object.Nil {
		t.Errorf("span of a group that took no part = %v", span)
	}
	if _, err := method(m, "span"); !errors.Is(err, object.TypeError) {
		t.Errorf("span() without key = %v", err)
	}
	d, err := method(m, "to_dict")
	if err != nil || show(d) != `{"key": "port", "value": nil}` {
		t.Fatalf("to_dict = %v, %v", show(d), err)
	}
	other, _ := method(m, "to_dict")
	if d == other {
		t.Fatal("to_dict returns a new Dict each time")
	}
	plain := mustCall(t, "find", object.Args{s(`(a)`), s("a")})
	if d, _ := method(plain, "to_dict"); show(d) != "{}" {
		t.Fatalf("unnamed to_dict = %s", show(d))
	}
}

// A name used twice: to_dict and m[name] give the last group that took part.
func TestDuplicateNames(t *testing.T) {
	pattern := s(`(?P<n>a)?(?P<n>b)?(?<n>c)?`)
	for text, want := range map[string]string{"ab": "b", "a": "a", "abc": "c", "": "nil"} {
		m := mustCall(t, "find", object.Args{pattern, s(text)})
		d, _ := method(m, "to_dict")
		got, _, _ := d.(*object.Dict).Get(s("n"))
		idx, _ := m.Index(s("n"))
		if show(got) != show(idx) || (want == "nil" && got != object.Nil) || (want != "nil" && got != s(want)) {
			t.Errorf("%q: to_dict n = %s, m[n] = %s, want %s", text, show(got), show(idx), want)
		}
	}
}

func TestFindAllAndSplit(t *testing.T) {
	all := mustCall(t, "find_all", object.Args{s(`#(\w+)`), s("#a b #cc #d")}).(*object.List)
	if len(all.Elements) != 3 || attr(t, all.Elements[2], "text") != s("#d") {
		t.Fatalf("find_all = %s", show(all))
	}
	two := mustCall(t, "find_all", object.Args{s(`#(\w+)`), s("#a b #cc #d")}, object.KwArg{Name: "count", Value: object.Integer(2)}).(*object.List)
	if len(two.Elements) != 2 {
		t.Fatalf("count=2 = %d", len(two.Elements))
	}
	none := mustCall(t, "find_all", object.Args{s(`z`), s("abc")})
	if show(none) != "[]" {
		t.Fatalf("no match = %s", show(none))
	}
	cases := []struct {
		count object.Object
		want  string
	}{{object.Nil, `["a", "b", "c"]`}, {object.Integer(1), `["a", "b,c"]`}, {object.Integer(0), `["a, b,c"]`}}
	for _, c := range cases {
		got := mustCall(t, "split", object.Args{s(`,\s*`), s("a, b,c")}, object.KwArg{Name: "count", Value: c.count})
		if show(got) != c.want {
			t.Errorf("split count=%v = %s", c.count, show(got))
		}
	}
	if got := mustCall(t, "split", object.Args{s(`;`), s("abc")}); show(got) != `["abc"]` {
		t.Fatalf("split no match = %s", show(got))
	}
	for _, name := range []string{"find_all", "split", "replace"} {
		args := object.Args{s(`a`), s("aaa")}
		if name == "replace" {
			args = append(args, s("b"))
		}
		_, err := callFn(name, args, object.KwArg{Name: "count", Value: object.Integer(-1)})
		if !errors.Is(err, object.ValueError) || err.Error() != name+"() argument 'count' must be non-negative, got -1" {
			t.Errorf("%s negative count = %v", name, err)
		}
	}
}

func TestReplace(t *testing.T) {
	cases := []struct{ pattern, text, replacement, want string }{
		{`\s+`, "a   b \t c", " ", "a b c"},
		{`(\d+)-(\d+)`, "10-20", "$2-$1", "20-10"},
		{`(?P<word>\w+)`, "hi", "<${word}>", "<hi>"},
		{`\d`, "a1", "$$", "a$"},
		{`(a)`, "a", "${1}x", "ax"},
	}
	for _, c := range cases {
		got := mustCall(t, "replace", object.Args{s(c.pattern), s(c.text), s(c.replacement)})
		if got != s(c.want) {
			t.Errorf("replace(%q, %q, %q) = %v", c.pattern, c.text, c.replacement, got)
		}
	}
	got := mustCall(t, "replace", object.Args{s(`a`), s("aaa"), s("b")}, object.KwArg{Name: "count", Value: object.Integer(2)})
	if got != s("bba") {
		t.Fatalf("count=2 = %v", got)
	}
	for template, msg := range map[string]string{
		"$x":   "replace() template refers to unknown group 'x'",
		"$2":   "replace() template refers to unknown group 2",
		"$1x":  "replace() template refers to unknown group '1x'; write ${...} to separate a group number from the text after it",
		"${y}": "replace() template refers to unknown group 'y'",
	} {
		_, err := callFn("replace", object.Args{s(`(a)`), s("a"), s(template)})
		if !errors.Is(err, object.ValueError) || err.Error() != msg {
			t.Errorf("template %q = %v", template, err)
		}
	}
	// A function receives each Match and returns the text to put there.
	angle := &object.Function{Name: "angle", Fn: func(args object.CallArgs) (object.Object, error) {
		text, _ := args.Positional[0].GetAttr("text")
		return s("<" + string(text.(object.String)) + ">"), nil
	}}
	if got := mustCall(t, "replace", object.Args{s(`\d+`), s("a1b22"), angle}); got != s("a<1>b<22>") {
		t.Fatalf("function replace = %v", got)
	}
	number := &object.Function{Name: "number", Fn: func(object.CallArgs) (object.Object, error) { return object.Integer(1), nil }}
	_, err := callFn("replace", object.Args{s(`\d`), s("a1"), number})
	if !errors.Is(err, object.TypeError) || err.Error() != "replace() replacement function must return String, got Int" {
		t.Fatalf("non-String result = %v", err)
	}
	boom := object.NewValueError("boom")
	failing := &object.Function{Name: "failing", Fn: func(object.CallArgs) (object.Object, error) { return nil, boom }}
	if _, err := callFn("replace", object.Args{s(`\d`), s("a1"), failing}); !errors.Is(err, boom) {
		t.Fatalf("raised error = %v", err)
	}
	_, err = callFn("replace", object.Args{s(`\d`), s("a1"), object.Integer(3)})
	if !errors.Is(err, object.TypeError) || err.Error() != "replace() argument 'replacement' must be String or callable, got Int" {
		t.Fatalf("bad replacement = %v", err)
	}
}

func TestInvalidPattern(t *testing.T) {
	for _, call := range []struct {
		name string
		args object.Args
		kws  []object.KwArg
	}{
		{"find", object.Args{s(`(`), s("")}, nil},
		{"find", object.Args{s(`(`), s("")}, []object.KwArg{{Name: "full", Value: object.True}}},
		{"find_all", object.Args{s(`(`), s("")}, nil},
		{"replace", object.Args{s(`(`), s(""), s("")}, nil},
		{"split", object.Args{s(`(`), s("")}, nil},
	} {
		for range 2 { // a failed compilation is not remembered
			_, err := callFn(call.name, call.args, call.kws...)
			var e *object.Error
			if !errors.As(err, &e) || !errors.Is(err, object.ParseError) {
				t.Fatalf("%s = %v", call.name, err)
			}
			if want := call.name + "() invalid pattern: missing closing ): `(`"; e.Error() != want {
				t.Errorf("message = %q, want %q", e.Error(), want)
			}
			if p, _ := e.Field("pattern"); p != s("(") {
				t.Errorf("pattern = %v", p)
			}
		}
	}
	if _, ok := cached(cacheKey{"(", false}); ok {
		t.Fatal("a failed pattern was cached")
	}
	_, err := callFn("find", object.Args{object.Bytes("a"), s("a")})
	if !errors.Is(err, object.TypeError) {
		t.Fatalf("Bytes pattern = %v", err)
	}
}

func TestCacheKeepsRecentPatterns(t *testing.T) {
	for i := range cacheSize + 10 {
		mustCall(t, "find", object.Args{s("p" + strconv.Itoa(i)), s("")})
	}
	cache.Lock()
	size := cache.order.Len()
	cache.Unlock()
	if size != cacheSize {
		t.Fatalf("cache holds %d patterns", size)
	}
	if _, ok := cached(cacheKey{"p0", false}); ok {
		t.Fatal("the oldest pattern was not evicted")
	}
	if _, ok := cached(cacheKey{"p" + strconv.Itoa(cacheSize+9), false}); !ok {
		t.Fatal("the newest pattern is missing")
	}
	mustCall(t, "find", object.Args{s("full"), s("full")}, object.KwArg{Name: "full", Value: object.True})
	if _, ok := cached(cacheKey{"full", true}); !ok {
		t.Fatal("the anchored form is cached under its own key")
	}
}

func TestEscapeAndConcurrency(t *testing.T) {
	needle := "(?i)" + string(mustCall(t, "escape", object.Args{s("a.c")}).(object.String))
	if mustCall(t, "find", object.Args{s(needle), s("abc")}) != object.Nil {
		t.Fatal("escaped dot matched any character")
	}
	if got := attr(t, mustCall(t, "find", object.Args{s(needle), s("xA.Cx")}), "text"); got != s("A.C") {
		t.Fatalf("escaped find = %v", got)
	}
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 200 {
				pattern := s("c" + strconv.Itoa((i*7+j)%300))
				if _, err := callFn("find", object.Args{pattern, s("")}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
