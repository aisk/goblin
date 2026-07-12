package extension

import (
	"fmt"

	"github.com/aisk/goblin/object"
)

var BuiltinsModule = &object.Module{
	Name: "builtin",
	Members: map[string]object.Object{
		"print":               &object.Function{Name: "print", Fn: print},
		"spawn":               &object.Function{Name: "spawn", Fn: spawn},
		"range":               &object.Function{Name: "range", Fn: range_},
		"max":                 &object.Function{Name: "max", Fn: max},
		"min":                 &object.Function{Name: "min", Fn: min},
		"Error":               object.ErrorConstructorFn,
		"TypeError":           object.TypeError,
		"ValueError":          object.ValueError,
		"LookupError":         object.LookupError,
		"ArithmeticError":     object.ArithmeticError,
		"IOError":             object.IOError,
		"ParseError":          object.ParseError,
		"IndexError":          object.IndexError,
		"KeyError":            object.KeyError,
		"ZeroDivisionError":   object.ZeroDivisionError,
		"AttributeError":      object.AttributeError,
		"NameError":           object.NameError,
		"ImportError":         object.ImportError,
		"NotExistError":       object.NotExistError,
		"ExistError":          object.ExistError,
		"PermissionError":     object.PermissionError,
		"TimeoutError":        object.TimeoutError,
		"NetworkError":        object.NetworkError,
		"NotImplementedError": object.NotImplementedError,
		"Int":                 object.IntConstructorFn,
		"Float":               object.FloatConstructorFn,
		"Str":                 object.StrConstructorFn,
		"Bool":                object.BoolConstructorFn,
		"List":                object.ListConstructorFn,
		"Dict":                object.DictConstructorFn,
		"Chan":                object.ChanConstructorFn,
		"Function":            object.FunctionConstructorFn,
	},
}

func print(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("print", args); err != nil {
		return nil, err
	}
	for i, arg := range args.Positional {
		if i > 0 {
			fmt.Print(" ")
		}
		s, err := arg.ToString()
		if err != nil {
			return nil, err
		}
		fmt.Print(s)
	}
	fmt.Print("\n")
	return nil, nil
}

// spawn launches a goblin function in a new goroutine, passing any extra
// positional arguments along to it. Goroutines are fire-and-forget: the
// function's return value and error are discarded, mirroring Go's `go`
// statement. Use a Chan to communicate results back.
func spawn(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("spawn", args)
	fn := p.Func("fn")
	rest := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	go fn.Call(object.CallArgs{Positional: rest})
	return object.Nil, nil
}

func range_(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("range", args)
	start, end := p.Int("start"), p.Int("end")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	if int64(start) >= int64(end) {
		return &object.List{Elements: []object.Object{}}, nil
	}

	elements := make([]object.Object, int64(end)-int64(start))
	for i := int64(start); i < int64(end); i++ {
		elements[i-int64(start)] = object.Integer(i)
	}

	return &object.List{Elements: elements}, nil
}

func max(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("max", args)
	nums := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(nums) == 0 {
		return nil, object.NewTypeError("max() requires at least 1 argument")
	}

	var hasFloat bool
	for _, arg := range nums {
		if _, ok := arg.(object.Float); ok {
			hasFloat = true
			break
		}
	}

	var maxValue float64
	if hasFloat {
		for i, arg := range nums {
			switch v := arg.(type) {
			case object.Float:
				if i == 0 || float64(v) > maxValue {
					maxValue = float64(v)
				}
			case object.Integer:
				if i == 0 || float64(v) > maxValue {
					maxValue = float64(v)
				}
			default:
				return nil, object.NewTypeError("max() argument %d: invalid type %T", i, arg)
			}
		}
		return object.Float(maxValue), nil
	}

	maxIntValue := int64(0)
	for i, arg := range nums {
		if v, ok := arg.(object.Integer); ok {
			if i == 0 || int64(v) > maxIntValue {
				maxIntValue = int64(v)
			}
		} else {
			return nil, object.NewTypeError("max() argument %d: invalid type %T", i, arg)
		}
	}
	return object.Integer(maxIntValue), nil
}

func min(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("min", args)
	nums := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(nums) == 0 {
		return nil, object.NewTypeError("min() requires at least 1 argument")
	}

	var hasFloat bool
	for _, arg := range nums {
		if _, ok := arg.(object.Float); ok {
			hasFloat = true
			break
		}
	}

	var minValue float64
	if hasFloat {
		for i, arg := range nums {
			switch v := arg.(type) {
			case object.Float:
				if i == 0 || float64(v) < minValue {
					minValue = float64(v)
				}
			case object.Integer:
				if i == 0 || float64(v) < minValue {
					minValue = float64(v)
				}
			default:
				return nil, object.NewTypeError("min() argument %d: invalid type %T", i, arg)
			}
		}
		return object.Float(minValue), nil
	}

	minIntValue := int64(0)
	for i, arg := range nums {
		if v, ok := arg.(object.Integer); ok {
			if i == 0 || int64(v) < minIntValue {
				minIntValue = int64(v)
			}
		} else {
			return nil, object.NewTypeError("min() argument %d: invalid type %T", i, arg)
		}
	}
	return object.Integer(minIntValue), nil
}
