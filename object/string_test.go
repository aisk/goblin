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
		{"contains_any", "goblin", "xyzl", True},
		{"count", "banana", "an", Integer(2)},
		{"equal_fold", "Goblin", "gObLiN", True},
		{"compare", "a", "b", Integer(-1)},
		{"index", "日本語-日本", "語", Integer(2)},
		{"last_index", "日本語-日本", "日", Integer(4)},
		{"index_any", "日本語", "語本", Integer(1)},
	}
	for _, tt := range tests {
		got := callStringMethod(t, tt.value, tt.method, Args{String(tt.arg)}, nil)
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestStringReplaceDefaultsAndKeywords(t *testing.T) {
	got := callStringMethod(t, "one one one", "replace", Args{String("one"), String("two")}, nil)
	if got != String("two two two") {
		t.Fatalf("default count: got %q", got)
	}
	got = callStringMethod(t, "one one one", "replace", nil, Kwargs{
		"old": String("one"), "new": String("two"), "count": Integer(2),
	})
	if got != String("two two one") {
		t.Fatalf("named count: got %q", got)
	}
}

func TestStringSplitOverloads(t *testing.T) {
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
	assertList(callStringMethod(t, "a,b,c", "split", Args{String(",")}, Kwargs{"count": Integer(2)}), String("a"), String("b,c"))
	assertList(callStringMethod(t, "a,b,c", "split_after", nil, Kwargs{"sep": String(",")}), String("a,"), String("b,"), String("c"))
	assertList(callStringMethod(t, "a,b,c", "split_after", Args{String(",")}, Kwargs{"count": Integer(2)}), String("a,"), String("b,c"))
	assertList(callStringMethod(t, " a\t b\n", "fields", nil, nil), String("a"), String("b"))
}

func TestStringTrimCutAndRepeat(t *testing.T) {
	if got := callStringMethod(t, "\u3000 hello \t", "trim", nil, nil); got != String("hello") {
		t.Fatalf("default whitespace trim: got %q", got)
	}
	if got := callStringMethod(t, "  hello  ", "trim_left", nil, nil); got != String("hello  ") {
		t.Fatalf("default whitespace trim_left: got %q", got)
	}
	if got := callStringMethod(t, "  hello  ", "trim", Args{String("")}, nil); got != String("  hello  ") {
		t.Fatalf("explicit empty cutset: got %q", got)
	}
	if got := callStringMethod(t, "xyhello", "trim_left", Args{String("xy")}, nil); got != String("hello") {
		t.Fatal(got)
	}
	if got := callStringMethod(t, "hello.go", "trim_suffix", nil, Kwargs{"suffix": String(".go")}); got != String("hello") {
		t.Fatal(got)
	}
	if got := callStringMethod(t, "go", "repeat", nil, Kwargs{"count": Integer(3)}); got != String("gogogo") {
		t.Fatal(got)
	}
	cut := callStringMethod(t, "key=value", "cut", Args{String("=")}, nil).(*List)
	if cut.Elements[0] != String("key") || cut.Elements[1] != String("value") || cut.Elements[2] != True {
		t.Fatal(cut)
	}
}

func TestStringMethodArgumentErrors(t *testing.T) {
	fn, _ := String("x").GetAttr("repeat")
	if _, err := Call(fn, CallArgs{Positional: Args{Integer(-1)}}); err == nil {
		t.Fatal("negative repeat count should fail")
	}
	if _, err := Call(fn, CallArgs{Keyword: Kwargs{"unknown": Integer(1)}}); err == nil {
		t.Fatal("unknown keyword should fail")
	}
}
