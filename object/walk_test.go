package object

import (
	"errors"
	"strings"
	"testing"
)

func ints(ns ...int) []Object {
	out := make([]Object, len(ns))
	for i, n := range ns {
		out[i] = Integer(n)
	}
	return out
}

// walk runs a pass over l, calling body after each element is taken, and
// returns the elements visited.
func walk(t *testing.T, l *List, body func()) []Object {
	t.Helper()
	it, err := Iterate(l)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	var seen []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return seen
		}
		if v == nil {
			t.Fatal("the walk handed out a Go nil")
		}
		seen = append(seen, v)
		body()
	}
}

func TestListWalkEndsWhenTheListShrinks(t *testing.T) {
	l := &List{Elements: ints(1, 2, 3)}
	seen := walk(t, l, func() {
		if len(l.Elements) > 1 {
			l.Pop(CallArgs{})
		}
	})
	if !(&List{Elements: seen}).equalInts(1, 2) {
		t.Fatalf("visited %v, want [1 2]", seen)
	}

	l = &List{Elements: ints(1, 2, 3)}
	seen = walk(t, l, func() { l.Pop(CallArgs{Positional: Args{Integer(0)}}) })
	if !(&List{Elements: seen}).equalInts(1, 3) {
		t.Fatalf("pop(0) visited %v, want [1 3]", seen)
	}

	l = &List{Elements: ints(1, 2, 3)}
	seen = walk(t, l, func() { l.Elements = append(l.Elements, Integer(0)) })
	if len(seen) != 3 || len(l.Elements) != 6 {
		t.Fatalf("pushed elements must not be visited: %v, list %v", seen, l)
	}
}

func (l *List) equalInts(ns ...int) bool {
	if len(l.Elements) != len(ns) {
		return false
	}
	for i, n := range ns {
		if l.Elements[i] != Integer(n) {
			return false
		}
	}
	return true
}

func TestListCallbacksFollowTheWalkRule(t *testing.T) {
	l := &List{Elements: ints(1, 2, 3, 4)}
	popper := &Function{Name: "f", Fn: func(args CallArgs) (Object, error) {
		l.Pop(CallArgs{})
		return args.Positional[0], nil
	}}
	got, err := l.mapMethod(CallArgs{Positional: Args{popper}})
	if err != nil || !got.(*List).equalInts(1, 2) {
		t.Fatalf("map = %v, %v", got, err)
	}
	l.Elements = ints(1, 2, 3, 4)
	if got, err = l.filterMethod(CallArgs{Positional: Args{popper}}); err != nil || !got.(*List).equalInts(1, 2) {
		t.Fatalf("filter = %v, %v", got, err)
	}
	l.Elements = ints(1, 2, 3, 4)
	if _, err = l.findMethod(CallArgs{Positional: Args{popper}}); err != nil {
		t.Fatal(err)
	}
	l.Elements = ints(1, 2, 3, 4)
	if s := l.String(); s != "[1, 2, 3, 4]" {
		t.Fatal(s)
	}
}

func TestSortCallsKeyOnceAndLeavesTheListOnFailure(t *testing.T) {
	calls := 0
	key := &Function{Name: "key", Fn: func(args CallArgs) (Object, error) {
		calls++
		return args.Positional[0], nil
	}}
	l := &List{Elements: ints(3, 1, 2, 5, 4)}
	if _, err := l.sortMethod(CallArgs{Keyword: Kwargs{{Name: "key", Value: key}}}); err != nil {
		t.Fatal(err)
	}
	if calls != 5 || !l.equalInts(1, 2, 3, 4, 5) {
		t.Fatalf("calls %d, list %v", calls, l)
	}

	failing := &Function{Name: "key", Fn: func(args CallArgs) (Object, error) {
		if args.Positional[0] == Integer(1) {
			return nil, NewValueError("bad key")
		}
		return args.Positional[0], nil
	}}
	l = &List{Elements: ints(3, 2, 1)}
	if _, err := l.sortMethod(CallArgs{Keyword: Kwargs{{Name: "key", Value: failing}}}); err == nil {
		t.Fatal("expected the key's error")
	}
	if !l.equalInts(3, 2, 1) {
		t.Fatalf("a failing key changed the list: %v", l)
	}

	l = &List{Elements: []Object{Integer(2), String("a"), Integer(1)}}
	if _, err := l.sortMethod(CallArgs{}); err == nil || l.Elements[0] != Integer(2) || l.Elements[2] != Integer(1) {
		t.Fatalf("a failing comparison changed the list: %v, %v", l, err)
	}

	clearing := &Function{Name: "key", Fn: func(args CallArgs) (Object, error) {
		l.Elements = nil
		return args.Positional[0], nil
	}}
	l = &List{Elements: ints(3, 1, 2)}
	if _, err := l.sortMethod(CallArgs{Keyword: Kwargs{{Name: "key", Value: clearing}}}); err != nil || !l.equalInts(3) {
		t.Fatalf("a clearing key: %v, %v", l, err)
	}
}

func TestUnpackAndListCopyAList(t *testing.T) {
	l := &List{Elements: ints(1, 2)}
	items, err := Unpack(l, 2)
	if err != nil {
		t.Fatal(err)
	}
	l.Pop(CallArgs{Positional: Args{Integer(0)}})
	if items[0] != Integer(1) || items[1] != Integer(2) {
		t.Fatalf("Unpack shares the list's storage: %v", items)
	}
	l = &List{Elements: ints(1, 2)}
	c, err := ListConstructor(CallArgs{Positional: Args{l}})
	if err != nil {
		t.Fatal(err)
	}
	c.(*List).Elements[0] = Integer(9)
	if l.Elements[0] != Integer(1) {
		t.Fatal("List(xs) shares xs's storage")
	}
}

func wantError(t *testing.T, err error, kind *Error, text string) {
	t.Helper()
	if err == nil || !errors.Is(err, kind) || err.Error() != text {
		t.Fatalf("got %v, want %s %q", err, kind.Value, text)
	}
}

func TestChanCapacityLimit(t *testing.T) {
	_, err := ChanConstructor(CallArgs{Positional: Args{Integer(MaxChanCapacity + 1)}})
	wantError(t, err, ValueError, "Chan() argument 'capacity' must be at most 1000000, got 1000001")
	_, err = ChanConstructor(CallArgs{Positional: Args{Integer(-1)}})
	wantError(t, err, ValueError, "Chan() argument 'capacity' must be non-negative, got -1")
	if _, err := ChanConstructor(CallArgs{Positional: Args{Integer(MaxChanCapacity)}}); err != nil {
		t.Fatal(err)
	}
}

func TestFloatTextOutOfRange(t *testing.T) {
	_, err := FloatConstructor(CallArgs{Positional: Args{String("1e400")}})
	wantError(t, err, OverflowError, `Float() value out of range: "1e400"`)
	_, err = FloatConstructor(CallArgs{Positional: Args{String("x")}})
	wantError(t, err, ValueError, `Float() invalid literal for Float: "x"`)
}

func TestKeyErrorShowsTheKeyAsAnElement(t *testing.T) {
	d := NewDict()
	_, err := d.Index(String("2"))
	wantError(t, err, KeyError, `key not found: "2"`)
	_, err = d.Index(Integer(2))
	wantError(t, err, KeyError, `key not found: 2`)
}

func TestDuplicateKeywordNamesTheCallee(t *testing.T) {
	var call CallArgs
	if err := call.AddKeyword("a", Integer(1)); err != nil {
		t.Fatal(err)
	}
	if err := call.UnpackKeywords(dictOf("a", Integer(2))); err != nil {
		t.Fatal(err)
	}
	_, err := BindArguments("f", []string{"a"}, nil, "", "", call)
	wantError(t, err, TypeError, "f() got multiple values for argument 'a'")
	_, err = BindArguments("g", nil, nil, "", "kw", call)
	wantError(t, err, TypeError, "g() got multiple values for argument 'a'")
	err = NewArgParser("split", call).Finish()
	wantError(t, err, TypeError, "split() got multiple values for argument 'a'")
	_, err = DictConstructor(call)
	wantError(t, err, TypeError, "Dict() got multiple values for argument 'a'")
}

func dictOf(key string, value Object) *Dict {
	d := NewDict()
	if err := d.Set(String(key), value); err != nil {
		panic(err)
	}
	return d
}

func TestCallAndRaiseNameTheType(t *testing.T) {
	_, err := Call(Integer(3), CallArgs{})
	wantError(t, err, TypeError, "Int is not callable")
	wantError(t, Raise(String("boom")), TypeError, "raise expects an Error, got String")
}

func TestSpawnMessagesNameNoParameter(t *testing.T) {
	_, err := Spawn(CallArgs{Keyword: Kwargs{{Name: "fn", Value: SpawnFn}}})
	wantError(t, err, TypeError, "spawn() takes the function to run as its first positional argument")
	_, err = Spawn(CallArgs{Positional: Args{Integer(1)}})
	wantError(t, err, TypeError, "spawn() first argument must be callable, got Int")
	if strings.Contains(err.Error(), "'fn'") {
		t.Fatal(err)
	}
}
