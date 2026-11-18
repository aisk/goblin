package object

import (
	"errors"
	"slices"
	"testing"
)

// This file is the conformance test for the members of the built-in types:
// the specification matrix STDLIB_DESIGN.md asks for. A new member of a
// built-in type goes into these tables first.

// members lists, per type, every member Attributes() reports, in its order.
var members = []struct {
	value Object
	names []string
}{
	{Integer(1), []string{"to_char"}},
	{Float(1), nil},
	{True, nil},
	{Nil, nil},
	{String("x"), []string{
		"size", "code_point", "upper", "lower", "trim", "trim_start", "trim_end", "trim_prefix",
		"trim_suffix", "has_prefix", "has_suffix", "contains", "count", "index",
		"replace", "split", "lines", "cut",
	}},
	{Bytes("x"), []string{
		"size", "decode", "to_hex", "contains", "count", "index", "has_prefix", "has_suffix",
		"trim_prefix", "trim_suffix", "replace", "split", "cut",
	}},
	{&List{}, []string{
		"size", "first", "last", "push", "pop", "insert", "remove", "clear",
		"reverse", "sort", "copy", "contains", "count", "index", "find", "join",
		"map", "filter", "reduce", "any", "all",
	}},
	{NewDict(), []string{
		"size", "keys", "values", "items", "contains", "get", "get_or_set", "pop",
		"remove", "update", "clear", "copy",
	}},
	{NewError("boom"), []string{"message", "traceback", "cause", "wrap", "is"}},
	{&Function{Name: "f"}, []string{"name"}},
	{NewChan(0), []string{"send", "receive", "close"}},
	{&Goblin{}, []string{"done", "wait"}},
}

func TestMatrixMembers(t *testing.T) {
	for _, m := range members {
		got := m.value.Attributes()
		if !slices.Equal(got, m.names) && (len(got) != 0 || len(m.names) != 0) {
			t.Errorf("%s members = %v\nwant %v", m.value.TypeName(), got, m.names)
		}
		for _, name := range got {
			if _, err := m.value.GetAttr(name); err != nil {
				t.Errorf("%s.%s: %v", m.value.TypeName(), name, err)
			}
		}
	}
}

// sharedOperations is the cross-type table: which of List, String, Bytes and
// Dict have each shared operation. Absent means a deliberate gap.
var sharedOperations = map[string][4]bool{
	//            List   String Bytes  Dict
	"size":     {true, true, true, true},
	"contains": {true, true, true, true},
	"count":    {true, true, true, false},
	"index":    {true, true, true, false},
	"find":     {true, false, false, false},
	"remove":   {true, false, false, true},
	"pop":      {true, false, false, true},
	"get":      {false, false, false, true},
	"replace":  {false, true, true, false},
	"split":    {false, true, true, false},
	"lines":    {false, true, false, false},
	"cut":      {false, true, true, false},
	"join":     {true, false, false, false},
	"copy":     {true, false, false, true},
	"clear":    {true, false, false, true},
	"to_hex":   {false, false, true, false},
}

func TestMatrixSharedOperations(t *testing.T) {
	values := [4]Object{&List{}, String(""), Bytes(""), NewDict()}
	for op, has := range sharedOperations {
		for i, v := range values {
			if got := slices.Contains(v.Attributes(), op); got != has[i] {
				t.Errorf("%s has %s = %v, want %v", v.TypeName(), op, got, has[i])
			}
		}
	}
}

// outcome is what a call should produce: a value (compared by inspect), the
// receiver itself, or an error of a kind with a message.
type outcome struct {
	value    string
	receiver bool
	kind     *Error
	message  string
}

func val(s string) outcome             { return outcome{value: s} }
func receiver() outcome                { return outcome{receiver: true} }
func fails(k *Error, m string) outcome { return outcome{kind: k, message: m} }

// repr renders a value the way a collection shows its elements, so strings
// are quoted.
func repr(v Object) string {
	s := inspect(&List{Elements: []Object{v}})
	return s[1 : len(s)-1]
}

func list(values ...Object) *List { return &List{Elements: values} }

func dict(pairs ...Object) *Dict {
	d := NewDict()
	for i := 0; i < len(pairs); i += 2 {
		d.Set(pairs[i], pairs[i+1])
	}
	return d
}

func pos(values ...Object) CallArgs { return CallArgs{Positional: values} }

func kw(pairs ...any) CallArgs {
	var args CallArgs
	for i := 0; i < len(pairs); i += 2 {
		args.Keyword = append(args.Keyword, KwArg{Name: pairs[i].(string), Value: pairs[i+1].(Object)})
	}
	return args
}

var identity = &Function{Name: "id", Fn: func(args CallArgs) (Object, error) { return args.Positional[0], nil }}
var never = &Function{Name: "never", Fn: func(CallArgs) (Object, error) { return False, nil }}
var add = &Function{Name: "add", Fn: func(args CallArgs) (Object, error) { return args.Positional[0].Add(args.Positional[1]) }}

// calls exercises every method with its keyword names (b), and pins the
// result on a miss, an empty receiver or an out-of-range position (c), and
// that mutating methods return the receiver (d). The receiver is built fresh
// for each call.
var calls = []struct {
	recv   func() Object
	method string
	args   CallArgs
	want   outcome
}{
	// Int
	{func() Object { return Integer(97) }, "to_char", CallArgs{}, val(`"a"`)},
	{func() Object { return Integer(0x1F600) }, "to_char", CallArgs{}, val(`"😀"`)},
	{func() Object { return Integer(0x10FFFF) }, "to_char", CallArgs{}, val(`"\U0010ffff"`)},
	{func() Object { return Integer(-1) }, "to_char", CallArgs{}, fails(ValueError, "to_char() code point out of range: -1")},
	{func() Object { return Integer(0x110000) }, "to_char", CallArgs{}, fails(ValueError, "to_char() code point out of range: 1114112")},
	{func() Object { return Integer(0xD800) }, "to_char", CallArgs{}, fails(ValueError, "to_char() code point 55296 is not a Unicode scalar value")},
	{func() Object { return Integer(65) }, "to_char", pos(Integer(1)), fails(TypeError, "to_char() takes 0 positional arguments, got 1")},

	// String
	{func() Object { return String("A") }, "code_point", CallArgs{}, val("65")},
	{func() Object { return String("é") }, "code_point", CallArgs{}, val("233")},
	{func() Object { return String("") }, "code_point", CallArgs{}, val("nil")},
	{func() Object { return String("abc") }, "code_point", CallArgs{}, val("97")},
	{func() Object { return String("\xff") }, "code_point", CallArgs{}, val("65533")},
	{func() Object { return String("aXa") }, "upper", CallArgs{}, val(`"AXA"`)},
	{func() Object { return String("aXa") }, "lower", CallArgs{}, val(`"axa"`)},
	{func() Object { return String("xax") }, "trim", kw("chars", String("x")), val(`"a"`)},
	{func() Object { return String(" a ") }, "trim", kw("chars", Nil), val(`"a"`)},
	{func() Object { return String(" a ") }, "trim", kw("chars", Integer(1)), fails(TypeError, "trim() argument 'chars' must be String or Nil, got Int")},
	{func() Object { return String("xax") }, "trim_start", kw("chars", String("x")), val(`"ax"`)},
	{func() Object { return String("xax") }, "trim_end", kw("chars", String("x")), val(`"xa"`)},
	{func() Object { return String("ab") }, "trim_prefix", kw("prefix", String("x")), val(`"ab"`)},
	{func() Object { return String("ab") }, "trim_suffix", kw("suffix", String("b")), val(`"a"`)},
	{func() Object { return String("ab") }, "has_prefix", kw("prefix", String("a")), val("true")},
	{func() Object { return String("ab") }, "has_suffix", kw("suffix", String("a")), val("false")},
	{func() Object { return String("ab") }, "contains", kw("value", String("c")), val("false")},
	{func() Object { return String("aaa") }, "count", kw("value", String("aa")), val("1")},
	{func() Object { return String("日a") }, "index", kw("value", String("a"), "start", Integer(-1)), val("1")},
	{func() Object { return String("ab") }, "index", kw("value", String("c")), val("nil")},
	{func() Object { return String("") }, "index", kw("value", String("")), val("0")},
	{func() Object { return String("ab") }, "index", kw("value", String("a"), "start", Integer(3)), fails(IndexError, "String index out of range: 3")},
	{func() Object { return String("aa") }, "replace", kw("old", String("a"), "new", String("b"), "count", Integer(1)), val(`"ba"`)},
	{func() Object { return String("aa") }, "replace", kw("old", String("a"), "new", String("b"), "count", Nil), val(`"bb"`)},
	{func() Object { return String("aa") }, "replace", kw("old", String("a"), "new", String("b"), "count", Integer(-1)), fails(ValueError, "replace() argument 'count' must be non-negative, got -1")},
	{func() Object { return String("abc") }, "replace", kw("old", String(""), "new", String("-")), fails(ValueError, "replace() argument 'old' must not be empty")},
	{func() Object { return String("a,b,c") }, "split", kw("sep", String(","), "count", Integer(1)), val(`["a", "b,c"]`)},
	{func() Object { return String(" a b ") }, "split", kw("sep", Nil), val(`["a", "b"]`)},
	{func() Object { return String("") }, "split", CallArgs{}, val(`[]`)},
	{func() Object { return String("ab") }, "split", kw("sep", String("")), fails(ValueError, "split() separator must not be empty")},
	{func() Object { return String("ab") }, "split", kw("count", Integer(-1)), fails(ValueError, "split() argument 'count' must be non-negative, got -1")},
	{func() Object { return String("a\nb\n") }, "lines", CallArgs{}, val(`["a", "b"]`)},
	{func() Object { return String("a\r\nb") }, "lines", CallArgs{}, val(`["a", "b"]`)},
	{func() Object { return String("a\rb\n\nc\r\n") }, "lines", CallArgs{}, val(`["a\rb", "", "c"]`)},
	{func() Object { return String("\n") }, "lines", CallArgs{}, val(`[""]`)},
	{func() Object { return String("") }, "lines", CallArgs{}, val(`[]`)},
	{func() Object { return String("a") }, "lines", pos(String("\n")), fails(TypeError, "lines() takes 0 positional arguments, got 1")},
	{func() Object { return String("a=b") }, "cut", kw("sep", String("=")), val(`["a", "b"]`)},
	{func() Object { return String("ab") }, "cut", kw("sep", String("=")), val(`["ab", nil]`)},

	// Bytes
	{func() Object { return Bytes("hé") }, "decode", CallArgs{}, val(`"hé"`)},
	{func() Object { return Bytes{0xff} }, "decode", CallArgs{}, fails(ValueError, "decode() invalid UTF-8")},
	{func() Object { return Bytes{0x00, 0xff, 0x10, 0xa4} }, "to_hex", CallArgs{}, val(`"00ff10a4"`)},
	{func() Object { return Bytes("") }, "to_hex", CallArgs{}, val(`""`)},
	{func() Object { return Bytes("a") }, "to_hex", pos(Integer(1)), fails(TypeError, "to_hex() takes 0 positional arguments, got 1")},
	{func() Object { return Bytes("ab") }, "contains", kw("value", String("b")), val("true")},
	{func() Object { return Bytes("aaa") }, "count", kw("value", Bytes("a")), val("3")},
	{func() Object { return Bytes("éa") }, "index", kw("value", String("a"), "start", Integer(1)), val("2")},
	{func() Object { return Bytes("ab") }, "index", kw("value", String("c")), val("nil")},
	{func() Object { return Bytes("ab") }, "index", kw("value", String("a"), "start", Integer(-3)), fails(IndexError, "Bytes index out of range: -3")},
	{func() Object { return Bytes("ab") }, "has_prefix", kw("prefix", String("a")), val("true")},
	{func() Object { return Bytes("ab") }, "has_suffix", kw("suffix", Bytes("b")), val("true")},
	{func() Object { return Bytes("ab") }, "trim_prefix", kw("prefix", String("x")), val(`b"ab"`)},
	{func() Object { return Bytes("ab") }, "trim_suffix", kw("suffix", String("b")), val(`b"a"`)},
	{func() Object { return Bytes("aa") }, "replace", kw("old", String("a"), "new", Bytes("b"), "count", Integer(1)), val(`b"ba"`)},
	{func() Object { return Bytes("aa") }, "replace", kw("old", String("a"), "new", Bytes("b"), "count", Integer(-2)), fails(ValueError, "replace() argument 'count' must be non-negative, got -2")},
	{func() Object { return Bytes("abc") }, "replace", kw("old", Bytes(""), "new", Bytes("-")), fails(ValueError, "replace() argument 'old' must not be empty")},
	{func() Object { return Bytes("a,b") }, "split", kw("sep", String(","), "count", Nil), val(`[b"a", b"b"]`)},
	{func() Object { return Bytes("a,b") }, "split", kw("sep", Bytes("")), fails(ValueError, "split() separator must not be empty")},
	{func() Object { return Bytes("a b") }, "split", CallArgs{}, fails(TypeError, "split() missing required argument: 'sep'")},
	{func() Object { return Bytes("a=b") }, "cut", kw("sep", String("=")), val(`[b"a", b"b"]`)},
	{func() Object { return Bytes("ab") }, "cut", kw("sep", String("=")), val(`[b"ab", nil]`)},

	// List
	{func() Object { return list() }, "first", CallArgs{}, val("nil")},
	{func() Object { return list() }, "last", CallArgs{}, val("nil")},
	{func() Object { return list() }, "push", pos(Integer(1)), receiver()},
	{func() Object { return list(Integer(1), Integer(2)) }, "pop", kw("index", Integer(0)), val("1")},
	{func() Object { return list() }, "pop", CallArgs{}, fails(IndexError, "pop() from empty list")},
	{func() Object { return list(Integer(1)) }, "pop", kw("index", Integer(1)), fails(IndexError, "List index out of range: 1")},
	{func() Object { return list() }, "insert", kw("index", Integer(0), "value", Integer(1)), receiver()},
	{func() Object { return list() }, "insert", kw("index", Integer(1), "value", Integer(1)), fails(IndexError, "List index out of range: 1")},
	{func() Object { return list(Integer(1)) }, "remove", kw("value", Integer(2)), receiver()},
	{func() Object { return list(Integer(1)) }, "clear", CallArgs{}, receiver()},
	{func() Object { return list(Integer(1)) }, "reverse", CallArgs{}, receiver()},
	{func() Object { return list(Integer(2), Integer(1)) }, "sort", kw("key", identity, "descending", True), receiver()},
	{func() Object { return list(Integer(1), String("a")) }, "sort", CallArgs{}, fails(TypeError, "cannot compare String and Int")},
	{func() Object { return list(Integer(1)) }, "copy", CallArgs{}, val("[1]")},
	{func() Object { return list(Integer(1)) }, "contains", kw("value", Integer(2)), val("false")},
	{func() Object { return list(Integer(1), Integer(1)) }, "count", kw("value", Integer(1)), val("2")},
	{func() Object { return list(Integer(1), Integer(1)) }, "index", kw("value", Integer(1), "start", Integer(-1)), val("1")},
	{func() Object { return list(Integer(1)) }, "index", kw("value", Integer(2)), val("nil")},
	{func() Object { return list(Integer(1)) }, "index", kw("value", Integer(1), "start", Integer(2)), fails(IndexError, "List index out of range: 2")},
	{func() Object { return list(Integer(1)) }, "find", kw("fn", never), val("nil")},
	{func() Object { return list(Integer(1), Integer(2)) }, "join", kw("sep", String("-")), val(`"1-2"`)},
	{func() Object { return list() }, "join", CallArgs{}, val(`""`)},
	{func() Object { return list() }, "join", kw("sep", Bytes("-")), val(`b""`)},
	{func() Object { return list(Bytes("a"), Integer(1)) }, "join", kw("sep", Bytes("-")), fails(TypeError, "join() element 1 must be Bytes, got Int")},
	{func() Object { return list(Integer(1)) }, "map", kw("fn", identity), val("[1]")},
	{func() Object { return list(Integer(1)) }, "filter", kw("fn", never), val("[]")},
	{func() Object { return list(Integer(1)) }, "map", kw("fn", Integer(1)), fails(TypeError, "map() argument 'fn' must be callable, got Int")},
	{func() Object { return list(Integer(1), Integer(2)) }, "reduce", kw("fn", add, "initial", Integer(10)), val("13")},
	{func() Object { return list() }, "reduce", kw("fn", add), fails(ValueError, "reduce() of empty list with no initial value")},
	{func() Object { return list() }, "any", kw("fn", Nil), val("false")},
	{func() Object { return list() }, "all", kw("fn", never), val("true")},

	// Dict
	{func() Object { return dict(String("a"), Integer(1)) }, "keys", CallArgs{}, val(`["a"]`)},
	{func() Object { return dict(String("a"), Integer(1)) }, "values", CallArgs{}, val(`[1]`)},
	{func() Object { return dict(String("a"), Integer(1)) }, "items", CallArgs{}, val(`[["a", 1]]`)},
	{func() Object { return dict() }, "contains", kw("key", String("a")), val("false")},
	{func() Object { return dict() }, "contains", kw("key", list()), fails(TypeError, "cannot hash List")},
	{func() Object { return dict() }, "get", kw("key", String("a")), val("nil")},
	{func() Object { return dict() }, "get", kw("key", String("a"), "default", Integer(0)), val("0")},
	{func() Object { return dict() }, "get_or_set", kw("key", String("a"), "default", Integer(0)), val("0")},
	{func() Object { return dict() }, "get_or_set", kw("key", String("a")), fails(TypeError, "get_or_set() missing required argument: 'default'")},
	{func() Object { return dict() }, "pop", kw("key", String("a"), "default", Integer(0)), val("0")},
	{func() Object { return dict() }, "pop", kw("key", String("a")), fails(KeyError, `key not found: "a"`)},
	{func() Object { return dict() }, "remove", kw("key", String("a")), receiver()},
	{func() Object { return dict() }, "update", kw("other", dict(String("a"), Integer(1))), receiver()},
	{func() Object { return dict() }, "update", kw("other", list()), fails(TypeError, "update() argument 'other' must be Dict, got List")},
	{func() Object { return dict(String("a"), Integer(1)) }, "clear", CallArgs{}, receiver()},
	{func() Object { return dict(String("b"), Integer(1), String("a"), Integer(2)) }, "copy", CallArgs{}, val(`{"b": 1, "a": 2}`)},

	// Error, Chan
	{func() Object { return NewError("boom") }, "wrap", kw("message", String("ctx")), val(`Error("ctx: boom")`)},
	{func() Object { return NewError("boom") }, "wrap", kw("message", Integer(1)), fails(TypeError, "wrap() argument 'message' must be String, got Int")},
	{func() Object { return NewError("boom") }, "is", kw("target", Integer(1)), fails(TypeError, "is() argument must be an Error, got Int")},
	{func() Object { c := NewChan(1); c.Close(CallArgs{}); return c }, "send", kw("value", Integer(1)), fails(ValueError, "send() on closed channel")},
	{func() Object { c := NewChan(1); c.Close(CallArgs{}); return c }, "receive", CallArgs{}, fails(ValueError, "receive() on closed channel")},
	{func() Object { c := NewChan(1); c.Close(CallArgs{}); return c }, "close", CallArgs{}, val("nil")},
	{func() Object { return NewChan(1) }, "close", CallArgs{}, val("nil")},
}

func TestMatrixCalls(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range calls {
		recv := c.recv()
		name := recv.TypeName() + "." + c.method
		covered[name] = true
		member, err := recv.GetAttr(c.method)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		got := member
		if fn, ok := member.(*Function); ok && c.method != "name" {
			got, err = Call(fn, c.args)
		}
		switch {
		case c.want.kind != nil:
			if !errors.Is(err, c.want.kind) || err.Error() != c.want.message {
				t.Errorf("%s%v: error %v, want %s: %s", name, c.args, err, c.want.kind, c.want.message)
			}
		case err != nil:
			t.Errorf("%s%v: %v", name, c.args, err)
		case c.want.receiver:
			if got != recv {
				t.Errorf("%s%v returned %v, want the receiver", name, c.args, got)
			}
		case repr(got) != c.want.value:
			t.Errorf("%s%v = %s, want %s", name, c.args, repr(got), c.want.value)
		}
	}
	// Every member of the collections and text types is pinned by a call.
	for _, m := range members[4:8] {
		for _, member := range m.names {
			if member == "size" {
				continue
			}
			if !covered[m.value.TypeName()+"."+member] {
				t.Errorf("no call covers %s.%s", m.value.TypeName(), member)
			}
		}
	}
}

// TestMatrixPositions pins what an out-of-range position does on each
// sequence type: an element index or a method's position names something
// that is not there and raises IndexError, while a slice takes the elements
// that exist between its ends.
func TestMatrixPositions(t *testing.T) {
	sequences := []struct {
		value func() Object
		size  int
	}{
		{func() Object { return list(Integer(1), Integer(2)) }, 2},
		{func() Object { return String("ab") }, 2},
		{func() Object { return Bytes("ab") }, 2},
	}
	for _, seq := range sequences {
		v := seq.value()
		name := v.TypeName()
		for _, i := range []Integer{2, -3} {
			if _, err := v.Index(i); !errors.Is(err, IndexError) {
				t.Errorf("%s[%d]: %v, want IndexError", name, i, err)
			}
		}
		slices := []struct {
			low, high Object
			want      int
		}{
			{nil, Integer(3), 2},
			{Integer(-10), nil, 2},
			{Integer(5), nil, 0},
			{nil, Integer(-10), 0},
			{Integer(1), Integer(0), 0},
			{Integer(-1), Integer(80), 1},
		}
		for _, sl := range slices {
			got, err := Slice(v, sl.low, sl.high)
			if err != nil {
				t.Errorf("%s[%v:%v]: %v", name, sl.low, sl.high, err)
				continue
			}
			size, _ := got.GetAttr("size")
			if size != Integer(sl.want) {
				t.Errorf("%s[%v:%v] has size %v, want %d", name, sl.low, sl.high, size, sl.want)
			}
		}
	}
}
