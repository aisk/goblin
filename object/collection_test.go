package object

import "testing"

func callMethod(t *testing.T, obj Object, name string, args CallArgs) Object {
	t.Helper()
	method, err := obj.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Call(method, args)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestListMethodsUseNamedAndDefaultArguments(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2), Integer(2), Integer(3)}}

	if got := callMethod(t, list, "index", CallArgs{Keyword: map[string]Object{"value": Integer(2), "start": Integer(2)}}); got != Integer(2) {
		t.Fatalf("index = %v, want 2", got)
	}
	if got := callMethod(t, list, "count", CallArgs{Positional: Args{Integer(2)}}); got != Integer(2) {
		t.Fatalf("count = %v, want 2", got)
	}
	if got := callMethod(t, list, "pop", CallArgs{}); got != Integer(3) {
		t.Fatalf("pop() = %v, want 3", got)
	}
	if got := callMethod(t, list, "pop", CallArgs{Keyword: map[string]Object{"index": Integer(-2)}}); got != Integer(2) {
		t.Fatalf("pop(index=-2) = %v, want 2", got)
	}
}

func TestListFirstAndLastDoNotMutate(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2), Integer(3)}}
	if got := callMethod(t, list, "first", CallArgs{}); got != Integer(1) {
		t.Fatalf("first = %v, want 1", got)
	}
	if got := callMethod(t, list, "last", CallArgs{}); got != Integer(3) {
		t.Fatalf("last = %v, want 3", got)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("first/last changed list length to %d", len(list.Elements))
	}
}

func TestChanSendRemainsPositionalOnly(t *testing.T) {
	channel := &Chan{ch: make(chan Object, 1)}
	method, err := channel.GetAttr("send")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Call(method, CallArgs{Keyword: Kwargs{"value": Integer(1)}})
	if err == nil {
		t.Fatal("send(value=...) should reject keyword arguments")
	}
}

func TestListMutationMethods(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(3)}}
	callMethod(t, list, "insert", CallArgs{Keyword: map[string]Object{"index": Integer(1), "value": Integer(2)}})
	if list.String() != "[1, 2, 3]" {
		t.Fatalf("insert result = %s", list)
	}
	if got := callMethod(t, list, "remove", CallArgs{Positional: Args{Integer(2)}}); got != True {
		t.Fatalf("remove = %v, want true", got)
	}
	copyObj := callMethod(t, list, "copy", CallArgs{}).(*List)
	callMethod(t, copyObj, "clear", CallArgs{})
	if len(list.Elements) != 2 || len(copyObj.Elements) != 0 {
		t.Fatal("copy and clear must not mutate the original list")
	}
}

func TestDictQueryMutationAndDefaults(t *testing.T) {
	dict := NewDict()
	dict.Set(String("a"), Integer(1))

	if got := callMethod(t, dict, "get", CallArgs{Keyword: map[string]Object{"key": String("missing"), "default": Integer(9)}}); got != Integer(9) {
		t.Fatalf("get default = %v, want 9", got)
	}
	if got := callMethod(t, dict, "set_default", CallArgs{Positional: Args{String("b"), Integer(2)}}); got != Integer(2) {
		t.Fatalf("set_default = %v, want 2", got)
	}
	if got := callMethod(t, dict, "pop", CallArgs{Keyword: map[string]Object{"key": String("b")}}); got != Integer(2) {
		t.Fatalf("pop = %v, want 2", got)
	}
	if got := callMethod(t, dict, "contains", CallArgs{Positional: Args{String("b")}}); got != False {
		t.Fatalf("contains removed key = %v, want false", got)
	}
	if got := callMethod(t, dict, "pop", CallArgs{Positional: Args{String("missing"), Nil}}); got != Nil {
		t.Fatalf("pop with explicit nil default = %v, want nil", got)
	}
	method, err := dict.GetAttr("pop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Call(method, CallArgs{Positional: Args{String("missing")}}); err == nil {
		t.Fatal("pop without a default should fail for a missing key")
	}
}

func TestConstructorsAcceptNamedDefaultArguments(t *testing.T) {
	tests := []struct {
		fn   *Function
		args CallArgs
		want string
	}{
		{IntConstructorFn, CallArgs{Keyword: map[string]Object{"value": String("12")}}, "12"},
		{FloatConstructorFn, CallArgs{Keyword: map[string]Object{"value": Integer(2)}}, "2"},
		{BoolConstructorFn, CallArgs{Keyword: map[string]Object{"value": String("")}}, "false"},
		{ListConstructorFn, CallArgs{Keyword: map[string]Object{"iterable": String("ab")}}, `["a", "b"]`},
	}
	for _, tt := range tests {
		got, err := tt.fn.Call(tt.args)
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != tt.want {
			t.Fatalf("%s() = %s, want %s", tt.fn.Name, got, tt.want)
		}
	}
}

func TestCollectionStringQuotesStringLiterals(t *testing.T) {
	list := &List{Elements: []Object{
		String("hello\nworld"),
		&List{Elements: []Object{String(`say "hi"`)}},
	}}
	if got, want := list.String(), `["hello\nworld", ["say \"hi\""]]`; got != want {
		t.Fatalf("List.String() = %q, want %q", got, want)
	}

	dict := NewDict()
	dict.Set(String("name"), String("Goblin"))
	if got, want := dict.String(), `{"name": "Goblin"}`; got != want {
		t.Fatalf("Dict.String() = %q, want %q", got, want)
	}
}

func TestListConstructorDistinguishesOmittedAndExplicitNil(t *testing.T) {
	if _, err := ListConstructor(CallArgs{}); err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if _, err := ListConstructor(CallArgs{Positional: Args{Nil}}); err == nil {
		t.Fatal("List(nil) should reject a non-iterable argument")
	}
}

func TestListFunctionalMethods(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2), Integer(3), Integer(4)}}

	// map
	gotMap := callMethod(t, list, "map", CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
		return args.Positional[0].(Integer) * 2, nil
	}}}})
	if gotMap.String() != "[2, 4, 6, 8]" {
		t.Fatalf("map result = %s", gotMap)
	}

	// filter
	gotFilter := callMethod(t, list, "filter", CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
		return Bool(args.Positional[0].(Integer)%2 == 0), nil
	}}}})
	if gotFilter.String() != "[2, 4]" {
		t.Fatalf("filter result = %s", gotFilter)
	}

	// reduce
	gotReduce := callMethod(t, list, "reduce", CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
		return args.Positional[0].(Integer) + args.Positional[1].(Integer), nil
	}}}})
	if gotReduce != Integer(10) {
		t.Fatalf("reduce result = %v, want 10", gotReduce)
	}

	// reduce with initial
	gotReduceInitial := callMethod(t, list, "reduce", CallArgs{
		Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
			return args.Positional[0].(Integer) + args.Positional[1].(Integer), nil
		}}},
		Keyword: Kwargs{"initial": Integer(10)},
	})
	if gotReduceInitial != Integer(20) {
		t.Fatalf("reduce with initial result = %v, want 20", gotReduceInitial)
	}

	// reduce with initial (positional)
	gotReduceInitialPos := callMethod(t, list, "reduce", CallArgs{
		Positional: Args{
			&Function{Fn: func(args CallArgs) (Object, error) {
				return args.Positional[0].(Integer) + args.Positional[1].(Integer), nil
			}},
			Integer(10),
		},
	})
	if gotReduceInitialPos != Integer(20) {
		t.Fatalf("reduce with positional initial result = %v, want 20", gotReduceInitialPos)
	}

	// find
	gotFind := callMethod(t, list, "find", CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
		return Bool(args.Positional[0].(Integer) > 2), nil
	}}}})
	if gotFind != Integer(3) {
		t.Fatalf("find result = %v, want 3", gotFind)
	}

	// any
	if got := callMethod(t, list, "any", CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
		return Bool(args.Positional[0].(Integer) > 3), nil
	}}}}); got != True {
		t.Fatalf("any result = %v, want true", got)
	}

	// all
	if got := callMethod(t, list, "all", CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) {
		return Bool(args.Positional[0].(Integer) > 0), nil
	}}}}); got != True {
		t.Fatalf("all result = %v, want true", got)
	}

	// sum
	if got := callMethod(t, list, "sum", CallArgs{}); got != Integer(10) {
		t.Fatalf("sum result = %v, want 10", got)
	}

	// reduce empty list without initial
	emptyList := &List{Elements: []Object{}}
	method, _ := emptyList.GetAttr("reduce")
	_, err := Call(method, CallArgs{Positional: Args{&Function{Fn: func(args CallArgs) (Object, error) { return Nil, nil }}}})
	if err == nil {
		t.Fatal("reduce() of empty list without initial should fail")
	}
}

func TestListSort(t *testing.T) {
	list := &List{Elements: []Object{Integer(3), Integer(1), Integer(4), Integer(2)}}
	callMethod(t, list, "sort", CallArgs{})
	if list.String() != "[1, 2, 3, 4]" {
		t.Fatalf("sort result = %s", list)
	}

	// reverse sort
	callMethod(t, list, "sort", CallArgs{Keyword: Kwargs{"reverse": True}})
	if list.String() != "[4, 3, 2, 1]" {
		t.Fatalf("reverse sort result = %s", list)
	}

	// sort with key
	list2 := &List{Elements: []Object{
		&List{Elements: []Object{Integer(2), String("b")}},
		&List{Elements: []Object{Integer(1), String("a")}},
	}}
	callMethod(t, list2, "sort", CallArgs{Keyword: Kwargs{"key": &Function{Fn: func(args CallArgs) (Object, error) {
		return args.Positional[0].(*List).Elements[0], nil
	}}}})
	if list2.Elements[0].(*List).Elements[0] != Integer(1) {
		t.Fatalf("sort with key result = %s", list2)
	}
}
