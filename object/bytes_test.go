package object

import "testing"

func callBytesMethod(t *testing.T, value Bytes, name string, args CallArgs) Object {
	t.Helper()
	fn, err := value.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Call(fn, args)
	if err != nil {
		t.Fatalf("%s() error: %v", name, err)
	}
	return got
}

func TestBytesConstructIndexIterateAndDecode(t *testing.T) {
	value, err := BytesConstructor(CallArgs{Positional: Args{String("hé")}})
	if err != nil {
		t.Fatal(err)
	}
	b := value.(Bytes)
	if len(b) != 3 {
		t.Fatalf("byte length = %d, want 3", len(b))
	}
	last, err := b.Index(Integer(2))
	if err != nil || last != Integer(0xa9) {
		t.Fatalf("last byte = %v, %v; want 169", last, err)
	}
	elements, _ := b.Iter()
	if len(elements) != 3 || elements[0] != Integer('h') {
		t.Fatalf("iteration = %#v", elements)
	}
	decoded, err := b.Decode(CallArgs{})
	if err != nil || decoded != String("hé") {
		t.Fatalf("decode = %v, %v", decoded, err)
	}
}

func TestBytesRejectInvalidValuesAndUTF8(t *testing.T) {
	_, err := BytesConstructor(CallArgs{Positional: Args{&List{Elements: []Object{Integer(256)}}}})
	if err == nil {
		t.Fatal("Bytes([256]) should fail")
	}
	if _, err := (Bytes{0xff}).Decode(CallArgs{}); err == nil {
		t.Fatal("decoding invalid UTF-8 should fail")
	}
}

func TestBytesSearchMethodsAcceptStringAndKeywords(t *testing.T) {
	b := Bytes("one two one")
	if got := callBytesMethod(t, b, "contains", CallArgs{Keyword: Kwargs{{Name: "value", Value: String("two")}}}); got != True {
		t.Fatalf("contains = %v", got)
	}
	if got := callBytesMethod(t, b, "count", CallArgs{Positional: Args{Bytes("one")}}); got != Integer(2) {
		t.Fatalf("count = %v", got)
	}
	if got := callBytesMethod(t, b, "index", CallArgs{Positional: Args{String("one")}, Keyword: Kwargs{{Name: "start", Value: Integer(1)}}}); got != Integer(8) {
		t.Fatalf("index = %v", got)
	}
	if got := callBytesMethod(t, Bytes("hé!"), "index", CallArgs{Positional: Args{String("!")}}); got != Integer(3) {
		t.Fatalf("index counts bytes: %v", got)
	}
	if got := callBytesMethod(t, b, "index", CallArgs{Positional: Args{String("x")}}); got != Nil {
		t.Fatalf("index miss = %v", got)
	}
}

func TestBytesReplaceSplitAndCut(t *testing.T) {
	replaced := callBytesMethod(t, Bytes("a-a-a"), "replace", CallArgs{
		Positional: Args{String("a"), Bytes("b")}, Keyword: Kwargs{{Name: "count", Value: Integer(2)}},
	}).(Bytes)
	if string(replaced) != "b-b-a" {
		t.Fatalf("replace = %q", replaced)
	}
	parts := callBytesMethod(t, Bytes("a,b,c"), "split", CallArgs{
		Keyword: Kwargs{{Name: "sep", Value: String(",")}, {Name: "count", Value: Integer(1)}},
	}).(*List)
	if len(parts.Elements) != 2 || string(parts.Elements[1].(Bytes)) != "b,c" {
		t.Fatalf("split = %#v", parts.Elements)
	}
	fn, _ := Bytes("x").GetAttr("split")
	if _, err := Call(fn, CallArgs{}); err == nil {
		t.Fatal("Bytes.split needs a separator")
	}
	cut := callBytesMethod(t, Bytes("key=value"), "cut", CallArgs{Keyword: Kwargs{{Name: "sep", Value: String("=")}}}).(*List)
	if len(cut.Elements) != 2 || string(cut.Elements[0].(Bytes)) != "key" || string(cut.Elements[1].(Bytes)) != "value" {
		t.Fatalf("cut = %#v", cut.Elements)
	}
	cut = callBytesMethod(t, Bytes("key"), "cut", CallArgs{Positional: Args{String("=")}}).(*List)
	if string(cut.Elements[0].(Bytes)) != "key" || cut.Elements[1] != Nil {
		t.Fatalf("cut miss = %#v", cut.Elements)
	}
	for _, name := range []string{"upper", "trim", "join", "fields", "to_valid_utf8", "index_byte"} {
		if _, err := Bytes("x").GetAttr(name); err == nil {
			t.Fatalf("Bytes still has %s", name)
		}
	}
}

func TestBytesHashByContent(t *testing.T) {
	d := NewDict()
	if err := d.Set(Bytes("k"), Integer(1)); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := d.Get(NewBytes([]byte("k"))); err != nil || !ok || v != Integer(1) {
		t.Fatalf("Get = %v, %v, %v", v, ok, err)
	}
	if _, ok, _ := d.Get(String("k")); ok {
		t.Fatal("String key found a Bytes entry")
	}
}
