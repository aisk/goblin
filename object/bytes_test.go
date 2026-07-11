package object

import "testing"

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
