package object

import (
	"fmt"
	"sort"
	"strings"
)

type List struct {
	Elements []Object
}

var _ Object = &List{}

func (l *List) Size(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("size", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("size() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return Integer(len(l.Elements)), nil
}

func (l *List) Push(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("push", args); err != nil {
		return nil, err
	}
	l.Elements = append(l.Elements, args.Positional...)
	return l, nil
}

func (l *List) Pop(args CallArgs) (Object, error) {
	ap := NewArgParser("pop", args)
	index := ap.IntOr("index", -1)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("pop from empty list")
	}
	i, err := listIndex("pop", index, len(l.Elements))
	if err != nil {
		return nil, err
	}
	value := l.Elements[i]
	copy(l.Elements[i:], l.Elements[i+1:])
	l.Elements = l.Elements[:len(l.Elements)-1]
	return value, nil
}

func (l *List) First(args CallArgs) (Object, error) {
	if err := requireNoArgs("first", args); err != nil {
		return nil, err
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("first() called on empty list")
	}
	return l.Elements[0], nil
}

func (l *List) Last(args CallArgs) (Object, error) {
	if err := requireNoArgs("last", args); err != nil {
		return nil, err
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("last() called on empty list")
	}
	return l.Elements[len(l.Elements)-1], nil
}

func listIndex(fn string, index Integer, size int) (int, error) {
	i := int(index)
	if i < 0 {
		i += size
	}
	if i < 0 || i >= size {
		return 0, NewIndexError("%s() index out of range: %d", fn, int64(index))
	}
	return i, nil
}

func (l *List) Join(args CallArgs) (Object, error) {
	ap := NewArgParser("join", args)
	sep := ap.Str("sep")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	elements := make([]string, len(l.Elements))
	for i, elem := range l.Elements {
		elements[i] = elem.String()
	}
	return String(strings.Join(elements, string(sep))), nil
}

func (l *List) Insert(args CallArgs) (Object, error) {
	ap := NewArgParser("insert", args)
	index, value := ap.Int("index"), ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	i := int(index)
	if i < 0 {
		i += len(l.Elements)
	}
	if i < 0 {
		i = 0
	}
	if i > len(l.Elements) {
		i = len(l.Elements)
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
	for _, elem := range l.Elements {
		if Equals(elem, value) {
			return True, nil
		}
	}
	return False, nil
}

func (l *List) Count(args CallArgs) (Object, error) {
	ap := NewArgParser("count", args)
	value := ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	count := 0
	for _, elem := range l.Elements {
		if Equals(elem, value) {
			count++
		}
	}
	return Integer(count), nil
}

func (l *List) IndexOf(args CallArgs) (Object, error) {
	ap := NewArgParser("index", args)
	value := ap.Any("value")
	start := ap.IntOr("start", 0)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	i := int(start)
	if i < 0 {
		i += len(l.Elements)
	}
	if i < 0 {
		i = 0
	}
	for ; i < len(l.Elements); i++ {
		if Equals(l.Elements[i], value) {
			return Integer(i), nil
		}
	}
	return Integer(-1), nil
}

func (l *List) Remove(args CallArgs) (Object, error) {
	ap := NewArgParser("remove", args)
	value := ap.Any("value")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	for i, elem := range l.Elements {
		if Equals(elem, value) {
			copy(l.Elements[i:], l.Elements[i+1:])
			l.Elements = l.Elements[:len(l.Elements)-1]
			return True, nil
		}
	}
	return False, nil
}

func (l *List) Reverse(args CallArgs) (Object, error) {
	if err := requireNoArgs("reverse", args); err != nil {
		return nil, err
	}
	for i, j := 0, len(l.Elements)-1; i < j; i, j = i+1, j-1 {
		l.Elements[i], l.Elements[j] = l.Elements[j], l.Elements[i]
	}
	return l, nil
}

func (l *List) Clear(args CallArgs) (Object, error) {
	if err := requireNoArgs("clear", args); err != nil {
		return nil, err
	}
	l.Elements = nil
	return l, nil
}

func (l *List) Copy(args CallArgs) (Object, error) {
	if err := requireNoArgs("copy", args); err != nil {
		return nil, err
	}
	elements := append([]Object(nil), l.Elements...)
	return &List{Elements: elements}, nil
}

func (l *List) String() string {
	elements := make([]string, len(l.Elements))
	for i, elem := range l.Elements {
		elements[i] = literal(elem)
	}
	return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
}

func (l *List) ToString() (string, error) { return l.String(), nil }

func (l *List) Bool() bool {
	return len(l.Elements) > 0
}

func (l *List) ToBool() (bool, error) { return l.Bool(), nil }

func (l *List) Compare(other Object) (int, error) {
	return 0, NewTypeError("cannot compare List and %T", other)
}

func (l *List) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case *List:
		newElements := make([]Object, len(l.Elements)+len(v.Elements))
		copy(newElements, l.Elements)
		copy(newElements[len(l.Elements):], v.Elements)
		return &List{Elements: newElements}, nil
	default:
		return nil, NewTypeError("cannot add List and %T", other)
	}
}

func (l *List) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from List")
}

func (l *List) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		if int64(v) < 0 {
			return nil, NewValueError("cannot multiply List by negative number")
		}
		newElements := make([]Object, len(l.Elements)*int(v))
		for i := 0; i < int(v); i++ {
			copy(newElements[i*len(l.Elements):], l.Elements)
		}
		return &List{Elements: newElements}, nil
	default:
		return nil, NewTypeError("cannot multiply List and %T", other)
	}
}

func (l *List) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide List")
}

func (l *List) And(other Object) (Object, error) {
	return Bool(l.Bool() && other.Bool()), nil
}

func (l *List) Or(other Object) (Object, error) {
	return Bool(l.Bool() || other.Bool()), nil
}

func (l *List) Not() (Object, error) {
	return Bool(!l.Bool()), nil
}

func (l *List) Iter() ([]Object, error) {
	return l.Elements, nil
}

func (l *List) Index(index Object) (Object, error) {
	idx, ok := index.(Integer)
	if !ok {
		return nil, NewTypeError("list index must be integer, got %T", index)
	}
	i := int(idx)
	if i < 0 || i >= len(l.Elements) {
		return nil, NewIndexError("list index out of range: %d", i)
	}
	return l.Elements[i], nil
}

func (l *List) SetIndex(index Object, value Object) error {
	idx, ok := index.(Integer)
	if !ok {
		return NewTypeError("list index must be integer, got %T", index)
	}
	i := int(idx)
	if i < 0 || i >= len(l.Elements) {
		return NewIndexError("list index out of range: %d", i)
	}
	l.Elements[i] = value
	return nil
}

func (l *List) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(l), nil
	case "size":
		return &Function{Name: "size", Fn: l.Size}, nil
	case "push":
		return &Function{Name: "push", Fn: l.Push}, nil
	case "pop":
		return &Function{Name: "pop", Fn: l.Pop}, nil
	case "first":
		return &Function{Name: "first", Fn: l.First}, nil
	case "last":
		return &Function{Name: "last", Fn: l.Last}, nil
	case "join":
		return &Function{Name: "join", Fn: l.Join}, nil
	case "insert":
		return &Function{Name: "insert", Fn: l.Insert}, nil
	case "contains":
		return &Function{Name: "contains", Fn: l.Contains}, nil
	case "count":
		return &Function{Name: "count", Fn: l.Count}, nil
	case "index":
		return &Function{Name: "index", Fn: l.IndexOf}, nil
	case "remove":
		return &Function{Name: "remove", Fn: l.Remove}, nil
	case "reverse":
		return &Function{Name: "reverse", Fn: l.Reverse}, nil
	case "clear":
		return &Function{Name: "clear", Fn: l.Clear}, nil
	case "copy":
		return &Function{Name: "copy", Fn: l.Copy}, nil
	case "sort":
		return &Function{Name: "sort", Fn: l.sortMethod}, nil
	case "map":
		return &Function{Name: "map", Fn: l.mapMethod}, nil
	case "filter":
		return &Function{Name: "filter", Fn: l.filterMethod}, nil
	case "reduce":
		return &Function{Name: "reduce", Fn: l.reduceMethod}, nil
	case "each":
		return &Function{Name: "each", Fn: l.eachMethod}, nil
	case "find":
		return &Function{Name: "find", Fn: l.findMethod}, nil
	case "any":
		return &Function{Name: "any", Fn: l.anyMethod}, nil
	case "all":
		return &Function{Name: "all", Fn: l.allMethod}, nil
	case "sum":
		return &Function{Name: "sum", Fn: l.sumMethod}, nil
	case "constructor":
		return ListConstructorFn, nil
	default:
		return nil, NewAttributeError("List has no attribute '%s'", name)
	}
}

func (l *List) sortMethod(args CallArgs) (Object, error) {
	p := NewArgParser("sort", args)
	key, supplied := p.OptionalAny("key")
	if !supplied {
		key = Nil
	}
	reverseObj, supplied := p.OptionalAny("reverse")
	reverse := false
	if supplied {
		reverse = reverseObj.Bool()
	}
	stableObj, supplied := p.OptionalAny("stable")
	stable := false
	if supplied {
		stable = stableObj.Bool()
	}
	if err := p.Finish(); err != nil {
		return nil, err
	}

	var sortErr error
	less := func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		var a, b Object
		if key != Nil {
			a, sortErr = Call(key, CallArgs{Positional: []Object{l.Elements[i]}})
			if sortErr != nil {
				return false
			}
			b, sortErr = Call(key, CallArgs{Positional: []Object{l.Elements[j]}})
			if sortErr != nil {
				return false
			}
		} else {
			a, b = l.Elements[i], l.Elements[j]
		}

		res, err := a.Compare(b)
		if err != nil {
			sortErr = err
			return false
		}
		if reverse {
			return res > 0
		}
		return res < 0
	}

	if stable {
		sort.SliceStable(l.Elements, less)
	} else {
		sort.Slice(l.Elements, less)
	}

	if sortErr != nil {
		return nil, sortErr
	}
	return Nil, nil
}

func (l *List) mapMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("map", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("map() takes exactly 1 argument, got %d", len(args.Positional))
	}
	fn := args.Positional[0]
	newElems := make([]Object, len(l.Elements))
	for i, e := range l.Elements {
		res, err := Call(fn, CallArgs{Positional: []Object{e}})
		if err != nil {
			return nil, err
		}
		newElems[i] = res
	}
	return &List{Elements: newElems}, nil
}

func (l *List) filterMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("filter", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("filter() takes exactly 1 argument, got %d", len(args.Positional))
	}
	fn := args.Positional[0]
	newElems := []Object{}
	for _, e := range l.Elements {
		res, err := Call(fn, CallArgs{Positional: []Object{e}})
		if err != nil {
			return nil, err
		}
		b, err := res.ToBool()
		if err != nil {
			return nil, err
		}
		if b {
			newElems = append(newElems, e)
		}
	}
	return &List{Elements: newElems}, nil
}

func (l *List) reduceMethod(args CallArgs) (Object, error) {
	p := NewArgParser("reduce", args)
	fn := p.Any("fn")
	initial, supplied := p.OptionalAny("initial")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	acc := initial
	start := 0
	if !supplied {
		if len(l.Elements) == 0 {
			return nil, NewTypeError("reduce() of empty list with no initial value")
		}
		acc = l.Elements[0]
		start = 1
	}
	for i := start; i < len(l.Elements); i++ {
		var err error
		acc, err = Call(fn, CallArgs{Positional: []Object{acc, l.Elements[i]}})
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func (l *List) eachMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("each", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("each() takes exactly 1 argument, got %d", len(args.Positional))
	}
	fn := args.Positional[0]
	for _, e := range l.Elements {
		_, err := Call(fn, CallArgs{Positional: []Object{e}})
		if err != nil {
			return nil, err
		}
	}
	return Nil, nil
}

func (l *List) findMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("find", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("find() takes exactly 1 argument, got %d", len(args.Positional))
	}
	fn := args.Positional[0]
	for _, e := range l.Elements {
		res, err := Call(fn, CallArgs{Positional: []Object{e}})
		if err != nil {
			return nil, err
		}
		b, err := res.ToBool()
		if err != nil {
			return nil, err
		}
		if b {
			return e, nil
		}
	}
	return Nil, nil
}

func (l *List) anyMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("any", args); err != nil {
		return nil, err
	}
	if len(args.Positional) > 1 {
		return nil, NewTypeError("any() takes at most 1 argument, got %d", len(args.Positional))
	}

	if len(args.Positional) == 0 {
		for _, e := range l.Elements {
			b, err := e.ToBool()
			if err != nil {
				return nil, err
			}
			if b {
				return True, nil
			}
		}
		return False, nil
	}

	fn := args.Positional[0]
	for _, e := range l.Elements {
		res, err := Call(fn, CallArgs{Positional: []Object{e}})
		if err != nil {
			return nil, err
		}
		b, err := res.ToBool()
		if err != nil {
			return nil, err
		}
		if b {
			return True, nil
		}
	}
	return False, nil
}

func (l *List) allMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("all", args); err != nil {
		return nil, err
	}
	if len(args.Positional) > 1 {
		return nil, NewTypeError("all() takes at most 1 argument, got %d", len(args.Positional))
	}

	if len(args.Positional) == 0 {
		for _, e := range l.Elements {
			b, err := e.ToBool()
			if err != nil {
				return nil, err
			}
			if !b {
				return False, nil
			}
		}
		return True, nil
	}

	fn := args.Positional[0]
	for _, e := range l.Elements {
		res, err := Call(fn, CallArgs{Positional: []Object{e}})
		if err != nil {
			return nil, err
		}
		b, err := res.ToBool()
		if err != nil {
			return nil, err
		}
		if !b {
			return False, nil
		}
	}
	return True, nil
}

func (l *List) sumMethod(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("sum", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("sum() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	var res Object = Integer(0)
	for _, e := range l.Elements {
		var err error
		res, err = res.Add(e)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (l *List) Attributes() []string {
	return []string{"attributes", "size", "push", "pop", "first", "last", "join", "insert", "contains", "count", "index", "remove", "reverse", "clear", "copy", "sort", "map", "filter", "reduce", "each", "find", "any", "all", "sum", "constructor"}
}

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
	elements, err := iterable.Iter()
	if err != nil {
		return nil, NewTypeError("List() argument is not iterable: %s", err)
	}
	return &List{Elements: elements}, nil
}
