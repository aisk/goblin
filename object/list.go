package object

import (
	"bytes"
	"cmp"
	"sort"
	"strings"
)

type List struct {
	NoReflectedOps
	NoAssignment
	Elements []Object
}

var _ Object = &List{}

func (l *List) Size() Integer {
	return Integer(len(l.Elements))
}

func (l *List) Push(args CallArgs) (Object, error) {
	ap := NewArgParser("push", args)
	values := ap.Rest()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	l.Elements = append(l.Elements, values...)
	return l, nil
}

func (l *List) Pop(args CallArgs) (Object, error) {
	ap := NewArgParser("pop", args)
	index := ap.IntOr("index", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("pop() from empty list")
	}
	i, err := ResolveIndex("List", index, len(l.Elements))
	if err != nil {
		return nil, err
	}
	value := l.Elements[i]
	copy(l.Elements[i:], l.Elements[i+1:])
	l.Elements[len(l.Elements)-1] = nil
	l.Elements = l.Elements[:len(l.Elements)-1]
	return value, nil
}

// First returns the first element, or nil for an empty list, exposed to
// Goblin as the read-only property list.first.
func (l *List) First() Object {
	if len(l.Elements) == 0 {
		return Nil
	}
	return l.Elements[0]
}

// Last returns the last element, or nil for an empty list, exposed to Goblin
// as the read-only property list.last.
func (l *List) Last() Object {
	if len(l.Elements) == 0 {
		return Nil
	}
	return l.Elements[len(l.Elements)-1]
}

// Join concatenates the elements with sep between them. A String separator
// joins the elements' text, converting each as print would; a Bytes
// separator joins elements that must all be Bytes.
func (l *List) Join(args CallArgs) (Object, error) {
	ap := NewArgParser("join", args)
	sepObj := ap.AnyOr("sep", String(""))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	switch sep := sepObj.(type) {
	case String:
		elements := make([]string, len(l.Elements))
		for i, elem := range l.Elements {
			s, err := elem.ToString()
			if err != nil {
				return nil, err
			}
			elements[i] = s
		}
		return String(strings.Join(elements, string(sep))), nil
	case Bytes:
		parts := make([][]byte, len(l.Elements))
		for i, elem := range l.Elements {
			part, ok := elem.(Bytes)
			if !ok {
				return nil, NewTypeError("join() element %d must be Bytes, got %s", i, elem.TypeName())
			}
			parts[i] = part
		}
		return Bytes(bytes.Join(parts, sep)), nil
	}
	return nil, NewTypeError("join() argument 'sep' must be String or Bytes, got %s", sepObj.TypeName())
}

// Insert puts value before the element at a bound, so index == size appends.
func (l *List) Insert(args CallArgs) (Object, error) {
	ap := NewArgParser("insert", args)
	index, value := ap.Int("index"), ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	i, err := ResolveBound("List", index, len(l.Elements))
	if err != nil {
		return nil, err
	}
	l.Elements = append(l.Elements, nil)
	copy(l.Elements[i+1:], l.Elements[i:])
	l.Elements[i] = value
	return l, nil
}

func (l *List) Contains(args CallArgs) (Object, error) {
	ap := NewArgParser("contains", args)
	value := ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	i, err := l.find(value, 0)
	if err != nil {
		return nil, err
	}
	return Bool(i >= 0), nil
}

// find returns the position of the first element from start on that equals
// value, or -1.
func (l *List) find(value Object, start int) (int, error) {
	for i := start; i < len(l.Elements); i++ {
		eq, err := Equals(l.Elements[i], value)
		if err != nil {
			return 0, err
		}
		if eq {
			return i, nil
		}
	}
	return -1, nil
}

func (l *List) Count(args CallArgs) (Object, error) {
	ap := NewArgParser("count", args)
	value := ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	count := 0
	for _, elem := range l.Elements {
		eq, err := Equals(elem, value)
		if err != nil {
			return nil, err
		}
		if eq {
			count++
		}
	}
	return Integer(count), nil
}

// IndexOf returns the position of the first element equal to value at or
// after the bound start, or nil when there is none.
func (l *List) IndexOf(args CallArgs) (Object, error) {
	ap := NewArgParser("index", args)
	value := ap.Any("value")
	start := ap.IntOr("start", 0)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	from, err := ResolveBound("List", start, len(l.Elements))
	if err != nil {
		return nil, err
	}
	i, err := l.find(value, from)
	if err != nil || i < 0 {
		return Nil, err
	}
	return Integer(i), nil
}

// Remove deletes the first element equal to value; without one it does
// nothing.
func (l *List) Remove(args CallArgs) (Object, error) {
	ap := NewArgParser("remove", args)
	value := ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	i, err := l.find(value, 0)
	if err != nil {
		return nil, err
	}
	if i >= 0 {
		copy(l.Elements[i:], l.Elements[i+1:])
		l.Elements[len(l.Elements)-1] = nil
		l.Elements = l.Elements[:len(l.Elements)-1]
	}
	return l, nil
}

func (l *List) Reverse(args CallArgs) (Object, error) {
	if err := RequireNoArgs("reverse", args); err != nil {
		return nil, err
	}
	for i, j := 0, len(l.Elements)-1; i < j; i, j = i+1, j-1 {
		l.Elements[i], l.Elements[j] = l.Elements[j], l.Elements[i]
	}
	return l, nil
}

func (l *List) Clear(args CallArgs) (Object, error) {
	if err := RequireNoArgs("clear", args); err != nil {
		return nil, err
	}
	l.Elements = nil
	return l, nil
}

func (l *List) Copy(args CallArgs) (Object, error) {
	if err := RequireNoArgs("copy", args); err != nil {
		return nil, err
	}
	elements := append([]Object(nil), l.Elements...)
	return &List{Elements: elements}, nil
}

func (l *List) TypeName() string { return "List" }

func (l *List) String() string {
	s, _ := (&renderer{}).list(l)
	return s
}

func (l *List) ToString() (string, error) { return (&renderer{strict: true}).list(l) }

func (l *List) ToBool() (bool, error) { return len(l.Elements) > 0, nil }

func (l *List) Equals(other Object) (bool, error) {
	v, ok := other.(*List)
	if !ok || len(l.Elements) != len(v.Elements) {
		return false, nil
	}
	// A list equals itself, which also settles `a == a` for a list that
	// contains itself. Distinct cyclic lists recurse until CheckStack stops
	// them.
	if l == v {
		return true, nil
	}
	if err := CheckStack(); err != nil {
		return false, err
	}
	for i, elem := range l.Elements {
		eq, err := Equals(elem, v.Elements[i])
		if err != nil || !eq {
			return false, err
		}
	}
	return true, nil
}

// Compare orders lists lexicographically: the first unequal pair of elements
// decides, and a list that is a prefix of the other sorts first.
func (l *List) Compare(other Object) (int, error) {
	v, ok := other.(*List)
	if !ok {
		return 0, NewOperandError(OpCompare, "List", other)
	}
	if err := CheckStack(); err != nil {
		return 0, err
	}
	for i := 0; i < len(l.Elements) && i < len(v.Elements); i++ {
		c, err := Compare(l.Elements[i], v.Elements[i])
		if err != nil || c != 0 {
			return c, err
		}
	}
	return cmp.Compare(len(l.Elements), len(v.Elements)), nil
}

func (l *List) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case *List:
		newElements := make([]Object, len(l.Elements)+len(v.Elements))
		copy(newElements, l.Elements)
		copy(newElements[len(l.Elements):], v.Elements)
		return &List{Elements: newElements}, nil
	default:
		return nil, NewOperandError(OpAdd, "List", other)
	}
}

func (l *List) Minus(other Object) (Object, error) {
	return nil, Unsupported("List", ErrFmtCannotSubtract)
}

func (l *List) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		if int64(v) < 0 {
			return nil, NewValueError("cannot multiply List by negative number")
		}
		if err := checkRepeat(len(l.Elements), v); err != nil {
			return nil, err
		}
		newElements := make([]Object, len(l.Elements)*int(v))
		for i := 0; i < int(v); i++ {
			copy(newElements[i*len(l.Elements):], l.Elements)
		}
		return &List{Elements: newElements}, nil
	default:
		return nil, NewOperandError(OpMultiply, "List", other)
	}
}

// RMultiply repeats the list when it is the right operand, so `2 * [1, 2]`
// reads the same as `[1, 2] * 2`.
func (l *List) RMultiply(left Object) (Object, bool, error) {
	if _, ok := left.(Integer); !ok {
		return nil, false, nil
	}
	result, err := l.Multiply(left)
	return result, true, err
}

func (l *List) Divide(other Object) (Object, error) {
	return nil, Unsupported("List", ErrFmtCannotDivide)
}

func (l *List) Modulo(other Object) (Object, error) {
	return nil, Unsupported("List", ErrFmtCannotModulo)
}

func (l *List) Iter() ([]Object, error) {
	return l.Elements, nil
}

func (l *List) Index(index Object) (Object, error) {
	// A non-negative in-range Int, the common case, skips the general
	// resolution, which keeps this small enough to inline into callers.
	if i, ok := index.(Integer); ok && i >= 0 && int64(i) < int64(len(l.Elements)) {
		return l.Elements[i], nil
	}
	return l.index(index)
}

func (l *List) index(index Object) (Object, error) {
	idx, err := indexArg("List", index)
	if err != nil {
		return nil, err
	}
	i, err := ResolveIndex("List", idx, len(l.Elements))
	if err != nil {
		return nil, err
	}
	return l.Elements[i], nil
}

func (l *List) SetIndex(index Object, value Object) (bool, error) {
	if i, ok := index.(Integer); ok && i >= 0 && int64(i) < int64(len(l.Elements)) {
		l.Elements[i] = value
		return true, nil
	}
	return l.setIndex(index, value)
}

func (l *List) setIndex(index Object, value Object) (bool, error) {
	idx, err := indexArg("List", index)
	if err != nil {
		return true, err
	}
	i, err := ResolveIndex("List", idx, len(l.Elements))
	if err != nil {
		return true, err
	}
	l.Elements[i] = value
	return true, nil
}

func (l *List) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return l.Size(), nil
	case "first":
		return l.First(), nil
	case "last":
		return l.Last(), nil
	}
	if fn, ok := listMethods.bind(l, name); ok {
		return fn, nil
	}
	return nil, NewAttributeError("List has no attribute '%s'", name)
}

// sortMethod sorts the list in place, stably: elements that order equal,
// or whose keys do, keep their relative order, descending included.
func (l *List) sortMethod(args CallArgs) (Object, error) {
	p := NewArgParser("sort", args)
	key, hasKey := p.NullableFunc("key")
	descending := p.BoolOr("descending", false)
	if err := p.Finish(); err != nil {
		return nil, err
	}

	var sortErr error
	argv := make(Args, 1)
	less := func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		a, b := l.Elements[i], l.Elements[j]
		if hasKey {
			argv[0] = a
			if a, sortErr = key.Call(CallArgs{Positional: argv}); sortErr != nil {
				return false
			}
			argv[0] = b
			if b, sortErr = key.Call(CallArgs{Positional: argv}); sortErr != nil {
				return false
			}
		}
		res, err := Compare(a, b)
		if err != nil {
			sortErr = err
			return false
		}
		if descending {
			return res > 0
		}
		return res < 0
	}
	sort.SliceStable(l.Elements, less)
	if sortErr != nil {
		return nil, sortErr
	}
	return l, nil
}

// callTruth calls fn on one element and reports the truth of its result.
func callTruth(fn *Function, argv Args, e Object) (bool, error) {
	argv[0] = e
	res, err := fn.Call(CallArgs{Positional: argv})
	if err != nil {
		return false, err
	}
	return res.ToBool()
}

func (l *List) mapMethod(args CallArgs) (Object, error) {
	ap := NewArgParser("map", args)
	fn := ap.Func("fn")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	newElems := make([]Object, len(l.Elements))
	argv := make(Args, 1)
	for i, e := range l.Elements {
		argv[0] = e
		res, err := fn.Call(CallArgs{Positional: argv})
		if err != nil {
			return nil, err
		}
		newElems[i] = res
	}
	return &List{Elements: newElems}, nil
}

func (l *List) filterMethod(args CallArgs) (Object, error) {
	ap := NewArgParser("filter", args)
	fn := ap.Func("fn")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	newElems := []Object{}
	argv := make(Args, 1)
	for _, e := range l.Elements {
		keep, err := callTruth(fn, argv, e)
		if err != nil {
			return nil, err
		}
		if keep {
			newElems = append(newElems, e)
		}
	}
	return &List{Elements: newElems}, nil
}

func (l *List) reduceMethod(args CallArgs) (Object, error) {
	p := NewArgParser("reduce", args)
	fn := p.Func("fn")
	initial, supplied := p.OptionalAny("initial")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	acc := initial
	start := 0
	if !supplied {
		if len(l.Elements) == 0 {
			return nil, NewValueError("reduce() of empty list with no initial value")
		}
		acc = l.Elements[0]
		start = 1
	}
	argv := make(Args, 2)
	for i := start; i < len(l.Elements); i++ {
		var err error
		argv[0], argv[1] = acc, l.Elements[i]
		acc, err = fn.Call(CallArgs{Positional: argv})
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func (l *List) findMethod(args CallArgs) (Object, error) {
	ap := NewArgParser("find", args)
	fn := ap.Func("fn")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	argv := make(Args, 1)
	for _, e := range l.Elements {
		found, err := callTruth(fn, argv, e)
		if err != nil {
			return nil, err
		}
		if found {
			return e, nil
		}
	}
	return Nil, nil
}

// anyAll answers any() (want true) or all() (want false): whether some
// element's truth, or that of fn's result for it, is want. Without fn the
// element itself is tested.
func (l *List) anyAll(name string, args CallArgs, want bool) (Object, error) {
	ap := NewArgParser(name, args)
	fn, hasFn := ap.NullableFunc("fn")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	argv := make(Args, 1)
	for _, e := range l.Elements {
		var b bool
		var err error
		if hasFn {
			b, err = callTruth(fn, argv, e)
		} else {
			b, err = e.ToBool()
		}
		if err != nil {
			return nil, err
		}
		if b == want {
			return Bool(want), nil
		}
	}
	return Bool(!want), nil
}

func (l *List) anyMethod(args CallArgs) (Object, error) { return l.anyAll("any", args, true) }

func (l *List) allMethod(args CallArgs) (Object, error) { return l.anyAll("all", args, false) }

func (l *List) Attributes() []string { return listMethods.attributes("size", "first", "last") }

var listMethods = newMethodTable(
	"push", "pop", "insert", "remove", "clear", "reverse", "sort", "copy",
	"contains", "count", "index", "find", "join", "map", "filter", "reduce",
	"any", "all",
)

var ListConstructorFn = &Function{Name: "List", Fn: ListConstructor}

func ListConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("List", args)
	iterable, supplied := ap.OptionalAny("iterable")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if !supplied {
		return &List{Elements: []Object{}}, nil
	}
	elements, err := CollectArg("List", "iterable", iterable)
	if err != nil {
		return nil, err
	}
	return &List{Elements: elements}, nil
}
