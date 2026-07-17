package object

import (
	"testing"
)

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
