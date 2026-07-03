package object

import (
	"fmt"
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
	if err := RequireNoKeyword("pop", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("pop() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("pop from empty list")
	}
	last := l.Elements[len(l.Elements)-1]
	l.Elements = l.Elements[:len(l.Elements)-1]
	return last, nil
}

func (l *List) First(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("first", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("first() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("first() called on empty list")
	}
	return l.Elements[0], nil
}

func (l *List) Last(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("last", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("last() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if len(l.Elements) == 0 {
		return nil, NewIndexError("last() called on empty list")
	}
	return l.Elements[len(l.Elements)-1], nil
}

func (l *List) Join(args CallArgs) (Object, error) {
	bound, err := BindArguments("join", []string{"sep"}, "", "", args)
	if err != nil {
		return nil, err
	}
	sep, ok := bound["sep"].(String)
	if !ok {
		return nil, NewTypeError("join() argument must be a string, got %T", bound["sep"])
	}
	elements := make([]string, len(l.Elements))
	for i, elem := range l.Elements {
		elements[i] = elem.String()
	}
	return String(strings.Join(elements, string(sep))), nil
}

func (l *List) String() string {
	elements := make([]string, len(l.Elements))
	for i, elem := range l.Elements {
		elements[i] = elem.String()
	}
	return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
}

func (l *List) Bool() bool {
	return len(l.Elements) > 0
}

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
	case "constructor":
		return ListConstructorFn, nil
	default:
		return nil, NewAttributeError("List has no attribute '%s'", name)
	}
}

var ListConstructorFn = &Function{Name: "List", Fn: ListConstructor}

func ListConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("List", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return &List{Elements: []Object{}}, nil
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("List() takes at most 1 argument, got %d", len(args.Positional))
	}
	elements, err := args.Positional[0].Iter()
	if err != nil {
		return nil, NewTypeError("List() argument is not iterable: %s", err)
	}
	return &List{Elements: elements}, nil
}
