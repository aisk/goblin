package object

import "testing"

func callStringMethod(t *testing.T, value, method string, positional Args, keyword Kwargs) Object {
	t.Helper()
	fn, err := String(value).GetAttr(method)
	if err != nil {
		t.Fatalf("GetAttr(%q): %v", method, err)
	}
	got, err := Call(fn, CallArgs{Positional: positional, Keyword: keyword})
	if err != nil {
		t.Fatalf("%s(): %v", method, err)
	}
	return got
}

func TestStringSearchMethodsUseRuneIndexes(t *testing.T) {
	tests := []struct {
		method, value, arg string
		want               Object
	}{
		{"contains", "goblin", "bl", True},
		{"count", "banana", "an", Integer(2)},
		{"count", "日本", "", Integer(3)},
		{"index", "日本語-日本", "語", Integer(2)},
		{"index", "日本語", "x", Nil},
	}
	for _, tt := range tests {
		got := callStringMethod(t, tt.value, tt.method, Args{String(tt.arg)}, nil)
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.method, got, tt.want)
		}
	}
	if got := callStringMethod(t, "日本語-日本", "index", Args{String("日")}, Kwargs{{Name: "start", Value: Integer(1)}}); got != Integer(4) {
		t.Errorf("index from 1 = %v, want 4", got)
	}
	if got := callStringMethod(t, "日本語-日本", "index", Args{String("本")}, Kwargs{{Name: "start", Value: Integer(-1)}}); got != Integer(5) {
		t.Errorf("index from -1 = %v, want 5", got)
	}
}

func TestStringReplaceDefaultsAndKeywords(t *testing.T) {
	got := callStringMethod(t, "one one one", "replace", Args{String("one"), String("two")}, nil)
	if got != String("two two two") {
		t.Fatalf("default count: got %q", got)
	}
	got = callStringMethod(t, "one one one", "replace", nil, Kwargs{
		{Name: "old", Value: String("one")},
		{Name: "new", Value: String("two")},
		{Name: "count", Value: Integer(2)},
	})
	if got != String("two two one") {
		t.Fatalf("named count: got %q", got)
	}
	fn, _ := String("x").GetAttr("replace")
	if _, err := Call(fn, CallArgs{Positional: Args{String("x"), String("y"), Integer(-1)}}); err == nil || err.Error() != "replace() argument 'count' must be non-negative, got -1" {
		t.Fatalf("negative count error = %v", err)
	}
}

func TestStringSplitCounts(t *testing.T) {
	assertList := func(got Object, want ...Object) {
		t.Helper()
		list := got.(*List)
		if len(list.Elements) != len(want) {
			t.Fatalf("got %v, want %v", list.Elements, want)
		}
		for i := range want {
			if list.Elements[i] != want[i] {
				t.Fatalf("got %v, want %v", list.Elements, want)
			}
		}
	}
	assertList(callStringMethod(t, "a,b,c", "split", Args{String(",")}, nil), String("a"), String("b"), String("c"))
	assertList(callStringMethod(t, "a,b,c", "split", Args{String(",")}, Kwargs{{Name: "count", Value: Integer(1)}}), String("a"), String("b,c"))
	assertList(callStringMethod(t, "a,b,c", "split", Args{String(",")}, Kwargs{{Name: "count", Value: Integer(0)}}), String("a,b,c"))
	assertList(callStringMethod(t, " a\t b\n", "split", nil, nil), String("a"), String("b"))
	assertList(callStringMethod(t, "  ", "split", nil, nil))
	assertList(callStringMethod(t, " a  b  c ", "split", Args{Nil, Integer(1)}, nil), String("a"), String("b  c "))
	assertList(callStringMethod(t, " a ", "split", Args{Nil, Integer(0)}, nil), String("a "))
	fn, _ := String("x").GetAttr("split")
	if _, err := Call(fn, CallArgs{Positional: Args{String("")}}); err == nil || err.Error() != "split() separator must not be empty" {
		t.Fatalf("empty separator error = %v", err)
	}
	if _, err := Call(fn, CallArgs{Positional: Args{String(","), Integer(-1)}}); err == nil || err.Error() != "split() argument 'count' must be non-negative, got -1" {
		t.Fatalf("negative count error = %v", err)
	}
}

func TestStringTrimAndCut(t *testing.T) {
	if got := callStringMethod(t, "\u3000 hello \t", "trim", nil, nil); got != String("hello") {
		t.Fatalf("default whitespace trim: got %q", got)
	}
	if got := callStringMethod(t, "  hello  ", "trim_start", nil, nil); got != String("hello  ") {
		t.Fatalf("default whitespace trim_start: got %q", got)
	}
	if got := callStringMethod(t, "  hello  ", "trim_end", nil, Kwargs{{Name: "chars", Value: Nil}}); got != String("  hello") {
		t.Fatalf("trim_end(chars=nil): got %q", got)
	}
	if got := callStringMethod(t, "  hello  ", "trim", Args{String("")}, nil); got != String("  hello  ") {
		t.Fatalf("explicit empty chars: got %q", got)
	}
	if got := callStringMethod(t, "xyhelloyx", "trim_start", nil, Kwargs{{Name: "chars", Value: String("xy")}}); got != String("helloyx") {
		t.Fatal(got)
	}
	if got := callStringMethod(t, "hello.go", "trim_suffix", nil, Kwargs{{Name: "suffix", Value: String(".go")}}); got != String("hello") {
		t.Fatal(got)
	}
	cut := callStringMethod(t, "key=value=x", "cut", Args{String("=")}, nil).(*List)
	if len(cut.Elements) != 2 || cut.Elements[0] != String("key") || cut.Elements[1] != String("value=x") {
		t.Fatal(cut)
	}
	cut = callStringMethod(t, "key", "cut", nil, Kwargs{{Name: "sep", Value: String("=")}}).(*List)
	if len(cut.Elements) != 2 || cut.Elements[0] != String("key") || cut.Elements[1] != Nil {
		t.Fatal(cut)
	}
}

func TestStringMethodArgumentErrors(t *testing.T) {
	fn, _ := String("x").GetAttr("contains")
	if _, err := Call(fn, CallArgs{Keyword: Kwargs{{Name: "sub", Value: String("x")}}}); err == nil {
		t.Fatal("the old keyword name should fail")
	}
	for _, name := range []string{"repeat", "encode", "fields", "trim_left", "last_index", "title"} {
		if _, err := String("x").GetAttr(name); err == nil {
			t.Fatalf("String still has %s", name)
		}
	}
}
