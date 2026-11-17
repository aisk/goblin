package csv

import (
	"errors"
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func call(name string, positional object.Args, keyword ...object.KwArg) (object.Object, error) {
	module, err := Execute()
	if err != nil {
		return nil, err
	}
	fn := module.(*object.Module).Members[name].(*object.Function)
	return fn.Call(object.CallArgs{Positional: positional, Keyword: keyword})
}

func strs(fields ...string) *object.List {
	elements := make([]object.Object, len(fields))
	for i, f := range fields {
		elements[i] = object.String(f)
	}
	return &object.List{Elements: elements}
}

func rows(elements ...object.Object) *object.List {
	return &object.List{Elements: elements}
}

func dict(pairs ...object.Object) *object.Dict {
	d := object.NewDict()
	for i := 0; i < len(pairs); i += 2 {
		d.Set(pairs[i], pairs[i+1])
	}
	return d
}

func show(t *testing.T, v object.Object) string {
	t.Helper()
	s, err := v.ToString()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestModuleMembers(t *testing.T) {
	module, _ := Execute()
	if got := strings.Join(module.(*object.Module).Attributes(), ","); got != "decode,decode_dicts,encode" {
		t.Fatalf("csv members = %s", got)
	}
}

func TestDecode(t *testing.T) {
	cases := []struct {
		in   string
		kw   []object.KwArg
		want string
	}{
		{"", nil, "[]"},
		{"\xef\xbb\xbfname,score\nAda,10\n", nil, `[["name", "score"], ["Ada", "10"]]`},
		{"a,\"b,c\"\n\n\"d\"\"e\",\"multi\nline\"\r\n", nil, `[["a", "b,c"], ["d\"e", "multi\nline"]]`},
		{"a,b,c\nd\ne,f,g,h\n", nil, `[["a", "b", "c"], ["d"], ["e", "f", "g", "h"]]`},
		{"a\tb\n", []object.KwArg{{Name: "sep", Value: object.String("\t")}}, `[["a", "b"]]`},
	}
	for _, c := range cases {
		got, err := call("decode", object.Args{object.String(c.in)}, c.kw...)
		if err != nil {
			t.Fatalf("decode(%q) error: %v", c.in, err)
		}
		if s := show(t, got); s != c.want {
			t.Errorf("decode(%q) = %s, want %s", c.in, s, c.want)
		}
	}
	got, err := call("decode", object.Args{object.NewBytes([]byte("x,y\n"))})
	if err != nil || show(t, got) != `[["x", "y"]]` {
		t.Fatalf("decode(Bytes) = %v, %v", got, err)
	}
}

func TestDecodeDicts(t *testing.T) {
	got, err := call("decode_dicts", object.Args{object.String("name,age\nAda,36\nLinus,28\n")})
	if err != nil {
		t.Fatal(err)
	}
	if s := show(t, got); s != `[{"name": "Ada", "age": "36"}, {"name": "Linus", "age": "28"}]` {
		t.Fatalf("decode_dicts = %s", s)
	}
	for _, in := range []string{"", "name,age\n"} {
		got, err := call("decode_dicts", object.Args{object.String(in)})
		if err != nil || show(t, got) != "[]" {
			t.Fatalf("decode_dicts(%q) = %v, %v", in, got, err)
		}
	}
}

func TestParseErrorsCarryPosition(t *testing.T) {
	cases := []struct {
		fn, in       string
		message      string
		line, column int64
	}{
		{"decode", "a,b\nc,d\"e\n", `decode() invalid CSV at line 2, column 4: stray quote`, 2, 4},
		{"decode", "é,ü\"x\n", `decode() invalid CSV at line 1, column 4: stray quote`, 1, 4},
		{"decode", "a,\"b\"x,c\n", `decode() invalid CSV at line 1, column 5: stray quote`, 1, 5},
		{"decode", "a,\"b\nc\n", `decode() invalid CSV at line 2, column 3: quoted field never ends`, 2, 3},
		{"decode_dicts", "id,name\n1,2\n3\n", `decode_dicts() invalid CSV at line 3, column 1: expected 2 fields, got 1`, 3, 1},
		{"decode_dicts", "id,\"x\ny\",z\n1,2,3\n4,5\n", `decode_dicts() invalid CSV at line 4, column 1: expected 3 fields, got 2`, 4, 1},
		{"decode_dicts", "id,näme,id\n", `decode_dicts() invalid CSV at line 1, column 9: duplicate column "id"`, 1, 9},
		{"decode_dicts", "x\n\"a\"\"\n", `decode_dicts() invalid CSV at line 2, column 6: quoted field never ends`, 2, 6},
	}
	for _, c := range cases {
		_, err := call(c.fn, object.Args{object.String(c.in)})
		var e *object.Error
		if !errors.As(err, &e) || !errors.Is(err, object.ParseError) {
			t.Fatalf("%s(%q) error = %v, want ParseError", c.fn, c.in, err)
		}
		if e.Error() != c.message {
			t.Errorf("%s(%q) message = %q, want %q", c.fn, c.in, e.Error(), c.message)
		}
		line, _ := e.Field("line")
		column, _ := e.Field("column")
		if line != object.Integer(c.line) || column != object.Integer(c.column) {
			t.Errorf("%s(%q) data = line %v, column %v", c.fn, c.in, line, column)
		}
	}
}

func TestSeparatorMustBeOneCharacter(t *testing.T) {
	for _, sep := range []string{"", ";;", "\"", "\n"} {
		_, err := call("decode", object.Args{object.String("a")}, object.KwArg{Name: "sep", Value: object.String(sep)})
		if err == nil || !errors.Is(err, object.ValueError) || !strings.Contains(err.Error(), "argument 'sep' must be one character") {
			t.Errorf("decode(sep=%q) error = %v", sep, err)
		}
	}
}

func TestEncode(t *testing.T) {
	columns := func(names ...string) object.KwArg { return object.KwArg{Name: "columns", Value: strs(names...)} }
	people := rows(
		dict(object.String("name"), object.String("Ada"), object.String("age"), object.Integer(36)),
		dict(object.String("age"), object.Integer(28), object.String("name"), object.String("Linus")),
		dict(object.String("name"), object.String("Grace")),
	)
	cases := []struct {
		rows object.Object
		kw   []object.KwArg
		want string
	}{
		{rows(), nil, ""},
		{rows(), []object.KwArg{columns("a", "b")}, "a,b\n"},
		{rows(strs("name", "note"), rows(object.String("Goblin"), object.String("a,b \"c\""), object.Integer(1), object.Float(2.5), object.True, object.Nil)), nil,
			"name,note\nGoblin,\"a,b \"\"c\"\"\",1,2.5,true,\n"},
		{people, nil, "name,age\nAda,36\nLinus,28\nGrace,\n"},
		{people, []object.KwArg{columns("age", "name")}, "age,name\n36,Ada\n28,Linus\n,Grace\n"},
		{rows(strs("1", "2")), []object.KwArg{columns("a", "b"), {Name: "sep", Value: object.String(";")}}, "a;b\n1;2\n"},
	}
	for _, c := range cases {
		got, err := call("encode", object.Args{c.rows}, c.kw...)
		if err != nil {
			t.Fatalf("encode(%v) error: %v", c.rows, err)
		}
		if string(got.(object.String)) != c.want {
			t.Errorf("encode(%v) = %q, want %q", c.rows, got, c.want)
		}
	}
}

func TestEncodeErrors(t *testing.T) {
	cases := []struct {
		rows    object.Object
		kw      []object.KwArg
		kind    *object.Error
		message string
	}{
		{rows(strs("a"), object.Integer(1)), nil, object.TypeError, "encode() row 2 must be List or Dict, got Int"},
		{rows(rows(strs("x"))), nil, object.TypeError, "encode() row 1 cannot hold a List as a field"},
		{rows(dict(object.String("a"), object.Integer(1)), dict(object.String("x"), object.Integer(2))), nil, object.ValueError, `encode() row 2 has unknown column "x"`},
		{rows(dict(object.String("a"), dict())), nil, object.TypeError, "encode() row 1 cannot hold a Dict as a field"},
		{rows(strs("a"), dict(object.String("a"), object.Integer(1))), nil, object.ValueError, "encode() row 2 is a Dict but the rows before it are Lists; give the columns with columns="},
		{object.Integer(3), nil, object.TypeError, "encode() argument 'rows' must be iterable, got Int"},
		{rows(), []object.KwArg{{Name: "columns", Value: object.String("a")}}, object.TypeError, "encode() argument 'columns' must be List or Nil, got String"},
	}
	for _, c := range cases {
		_, err := call("encode", object.Args{c.rows}, c.kw...)
		if err == nil || !errors.Is(err, c.kind) || err.Error() != c.message {
			t.Errorf("encode(%v) error = %v, want %q", c.rows, err, c.message)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	table := rows(strs("name", "note"), strs("Goblin", "a,b\nc \"d\""), strs("", " x"))
	text, err := call("encode", object.Args{table})
	if err != nil {
		t.Fatal(err)
	}
	back, err := call("decode", object.Args{text})
	if err != nil {
		t.Fatal(err)
	}
	if show(t, back) != show(t, table) {
		t.Fatalf("round trip = %s", show(t, back))
	}
}
