package object

import (
	"strings"
)

type String string

var _ Object = String("")

func (s String) Size(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("size", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("size() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return Integer(len([]rune(string(s)))), nil
}

func (s String) Upper(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("upper", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("upper() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return String(strings.ToUpper(string(s))), nil
}

func (s String) Lower(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("lower", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("lower() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return String(strings.ToLower(string(s))), nil
}

func (s String) HasPrefix(args CallArgs) (Object, error) {
	ap := NewArgParser("has_prefix", args)
	prefix := ap.Str("prefix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.HasPrefix(string(s), string(prefix))), nil
}

func (s String) HasSuffix(args CallArgs) (Object, error) {
	ap := NewArgParser("has_suffix", args)
	suffix := ap.Str("suffix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.HasSuffix(string(s), string(suffix))), nil
}

func (s String) Trim(args CallArgs) (Object, error) {
	ap := NewArgParser("trim", args)
	cutset := ap.Str("cutset")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return String(strings.Trim(string(s), string(cutset))), nil
}

func (s String) TrimSpace(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("trim_space", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("trim_space() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return String(strings.TrimSpace(string(s))), nil
}

func (s String) Contains(args CallArgs) (Object, error) {
	ap := NewArgParser("contains", args)
	substr := ap.Str("substr")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return Bool(strings.Contains(string(s), string(substr))), nil
}

func (s String) String() string {
	return string(s)
}

func (s String) Bool() bool {
	if s == "" {
		return false
	}
	return true
}

func (s String) Compare(other Object) (int, error) {
	switch v := other.(type) {
	case String:
		a, b := string(s), string(v)
		if a < b {
			return -1, nil
		}
		if a > b {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare String and %T", other)
	}
}

func (s String) Add(other Object) (Object, error) {
	switch v := other.(type) {
	case String:
		return String(string(s) + string(v)), nil
	case Integer:
		return String(string(s) + v.String()), nil
	case Bool:
		return String(string(s) + v.String()), nil
	default:
		return nil, NewTypeError("cannot add String and %T", other)
	}
}

func (s String) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from String")
}

func (s String) Multiply(other Object) (Object, error) {
	switch v := other.(type) {
	case Integer:
		result := ""
		for i := int64(0); i < int64(v); i++ {
			result += string(s)
		}
		return String(result), nil
	default:
		return nil, NewTypeError("cannot multiply String and %T", other)
	}
}

func (s String) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide String")
}

func (s String) And(other Object) (Object, error) {
	return Bool(s.Bool() && other.Bool()), nil
}

func (s String) Or(other Object) (Object, error) {
	return Bool(s.Bool() || other.Bool()), nil
}

func (s String) Not() (Object, error) {
	return Bool(!s.Bool()), nil
}

func (s String) Iter() ([]Object, error) {
	// String can be iterated character by character
	var result []Object
	for _, char := range string(s) {
		result = append(result, String(string(char)))
	}
	return result, nil
}

func (s String) Index(index Object) (Object, error) {
	return nil, NewTypeError("String is not indexable")
}

func (s String) GetAttr(name string) (Object, error) {
	switch name {
	case "size":
		return &Function{Name: "size", Fn: s.Size}, nil
	case "upper":
		return &Function{Name: "upper", Fn: s.Upper}, nil
	case "lower":
		return &Function{Name: "lower", Fn: s.Lower}, nil
	case "has_prefix":
		return &Function{Name: "has_prefix", Fn: s.HasPrefix}, nil
	case "has_suffix":
		return &Function{Name: "has_suffix", Fn: s.HasSuffix}, nil
	case "trim":
		return &Function{Name: "trim", Fn: s.Trim}, nil
	case "trim_space":
		return &Function{Name: "trim_space", Fn: s.TrimSpace}, nil
	case "contains":
		return &Function{Name: "contains", Fn: s.Contains}, nil
	case "constructor":
		return StrConstructorFn, nil
	default:
		return nil, NewAttributeError("String has no attribute '%s'", name)
	}
}

var StrConstructorFn = &Function{Name: "Str", Fn: StrConstructor}

func StrConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Str", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return String(""), nil
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("Str() takes at most 1 argument, got %d", len(args.Positional))
	}
	s, err := Repr(args.Positional[0])
	if err != nil {
		return nil, err
	}
	return String(s), nil
}
