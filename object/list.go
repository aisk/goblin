package object

import (
	"fmt"
	"strings"
)

type List struct {
	Elements []Object
}

func (l *List) Repr() string {
	elements := make([]string, len(l.Elements))
	for i, elem := range l.Elements {
		elements[i] = elem.Repr()
	}
	return fmt.Sprintf("object.List([%s])", strings.Join(elements, ", "))
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
	return 0, fmt.Errorf("cannot compare List and %T", other)
}

func (l *List) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case *List:
		newElements := make([]Object, len(l.Elements)+len(v.Elements))
		copy(newElements, l.Elements)
		copy(newElements[len(l.Elements):], v.Elements)
		return &List{Elements: newElements}, nil
	default:
		return nil, fmt.Errorf("cannot add List and %T", other)
	}
}

func (l *List) Minus(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot subtract from List")
}

func (l *List) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		if int64(v) < 0 {
			return nil, fmt.Errorf("cannot multiply List by negative number")
		}
		newElements := make([]Object, len(l.Elements)*int(v))
		for i := 0; i < int(v); i++ {
			copy(newElements[i*len(l.Elements):], l.Elements)
		}
		return &List{Elements: newElements}, nil
	default:
		return nil, fmt.Errorf("cannot multiply List and %T", other)
	}
}

func (l *List) Divide(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot divide List")
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
		return nil, fmt.Errorf("list index must be integer, got %T", index)
	}
	i := int(idx)
	if i < 0 || i >= len(l.Elements) {
		return nil, fmt.Errorf("list index out of range: %d", i)
	}
	return l.Elements[i], nil
}

func (l *List) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return Integer(len(l.Elements)), nil
	case "push":
		return &Function{Name: "push", Fn: func(args CallArgs) (Object, error) {
			if err := RequireNoKeyword("push", args); err != nil {
				return nil, err
			}
			l.Elements = append(l.Elements, args.Positional...)
			return l, nil
		}}, nil
	case "pop":
		return &Function{Name: "pop", Fn: func(args CallArgs) (Object, error) {
			if err := RequireNoKeyword("pop", args); err != nil {
				return nil, err
			}
			if len(args.Positional) != 0 {
				return nil, fmt.Errorf("pop() takes exactly 0 arguments, got %d", len(args.Positional))
			}
			if len(l.Elements) == 0 {
				return nil, fmt.Errorf("pop from empty list")
			}
			last := l.Elements[len(l.Elements)-1]
			l.Elements = l.Elements[:len(l.Elements)-1]
			return last, nil
		}}, nil
	case "first":
		return &Function{Name: "first", Fn: func(args CallArgs) (Object, error) {
			if err := RequireNoKeyword("first", args); err != nil {
				return nil, err
			}
			if len(args.Positional) != 0 {
				return nil, fmt.Errorf("first() takes exactly 0 arguments, got %d", len(args.Positional))
			}
			if len(l.Elements) == 0 {
				return nil, fmt.Errorf("first() called on empty list")
			}
			return l.Elements[0], nil
		}}, nil
	case "last":
		return &Function{Name: "last", Fn: func(args CallArgs) (Object, error) {
			if err := RequireNoKeyword("last", args); err != nil {
				return nil, err
			}
			if len(args.Positional) != 0 {
				return nil, fmt.Errorf("last() takes exactly 0 arguments, got %d", len(args.Positional))
			}
			if len(l.Elements) == 0 {
				return nil, fmt.Errorf("last() called on empty list")
			}
			return l.Elements[len(l.Elements)-1], nil
		}}, nil
	case "join":
		return &Function{Name: "join", Fn: func(args CallArgs) (Object, error) {
			bound, err := BindArguments("join", []string{"sep"}, "", "", args)
			if err != nil {
				return nil, err
			}
			sep, ok := bound["sep"].(String)
			if !ok {
				return nil, fmt.Errorf("join() argument must be a string, got %T", bound["sep"])
			}
			elements := make([]string, len(l.Elements))
			for i, elem := range l.Elements {
				elements[i] = elem.String()
			}
			return String(strings.Join(elements, string(sep))), nil
		}}, nil
	default:
		return nil, fmt.Errorf("List has no attribute '%s'", name)
	}
}

var _ Object = &List{}
