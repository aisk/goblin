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
	last, err := b.Index(Integer(-1))
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

func TestStringEncode(t *testing.T) {
	fn, err := String("hi").GetAttr("encode")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Call(fn, CallArgs{})
	if err != nil || string(got.(Bytes)) != "hi" {
		t.Fatalf("encode = %v, %v", got, err)
	}
}

func TestBytesSearchMethodsAcceptStringAndKeywords(t *testing.T) {
	b := Bytes("one two one")
	if got := callBytesMethod(t, b, "contains", CallArgs{Keyword: Kwargs{"sub": String("two")}}); got != True {
		t.Fatalf("contains = %v", got)
	}
	if got := callBytesMethod(t, b, "count", CallArgs{Positional: Args{Bytes("one")}}); got != Integer(2) {
		t.Fatalf("count = %v", got)
	}
	if got := callBytesMethod(t, b, "last_index", CallArgs{Keyword: Kwargs{"sub": String("one")}}); got != Integer(8) {
		t.Fatalf("last_index = %v", got)
	}
}

func TestBytesReplaceSplitAndTrimDefaults(t *testing.T) {
	replaced := callBytesMethod(t, Bytes("a-a-a"), "replace", CallArgs{
		Positional: Args{String("a"), Bytes("b")}, Keyword: Kwargs{"count": Integer(2)},
	}).(Bytes)
	if string(replaced) != "b-b-a" {
		t.Fatalf("replace = %q", replaced)
	}
	parts := callBytesMethod(t, Bytes("a,b,c"), "split", CallArgs{
		Keyword: Kwargs{"sep": String(","), "count": Integer(2)},
	}).(*List)
	if len(parts.Elements) != 2 || string(parts.Elements[1].(Bytes)) != "b,c" {
		t.Fatalf("split = %#v", parts.Elements)
	}
	trimmed := callBytesMethod(t, Bytes(" \tvalue\n"), "trim", CallArgs{}).(Bytes)
	if string(trimmed) != "value" {
		t.Fatalf("trim = %q", trimmed)
	}
}

func TestBytesCutCaseAndValidUTF8(t *testing.T) {
	cut := callBytesMethod(t, Bytes("key=value"), "cut", CallArgs{Keyword: Kwargs{"sep": String("=")}}).(*List)
	if string(cut.Elements[0].(Bytes)) != "key" || string(cut.Elements[1].(Bytes)) != "value" || cut.Elements[2] != True {
		t.Fatalf("cut = %#v", cut.Elements)
	}
	upper := callBytesMethod(t, Bytes("hé"), "upper", CallArgs{}).(Bytes)
	if string(upper) != "HÉ" {
		t.Fatalf("upper = %q", upper)
	}
	valid := callBytesMethod(t, Bytes{'a', 0xff, 'b'}, "to_valid_utf8", CallArgs{
		Keyword: Kwargs{"replacement": String("?")},
	}).(Bytes)
	if string(valid) != "a?b" {
		t.Fatalf("to_valid_utf8 = %q", valid)
	}
}
