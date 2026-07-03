package extension

import (
	"fmt"

	"github.com/aisk/goblin/object"
)

var BuiltinsModule = &object.Module{
	Name: "builtin",
	Members: map[string]object.Object{
		"print":             &object.Function{Name: "print", Fn: print},
		"spawn":             &object.Function{Name: "spawn", Fn: spawn},
		"range":             &object.Function{Name: "range", Fn: range_},
		"max":               &object.Function{Name: "max", Fn: max},
		"min":               &object.Function{Name: "min", Fn: min},
		"Error":             object.ErrorConstructorFn,
		"TypeError":         object.TypeError,
		"ValueError":        object.ValueError,
		"IndexError":        object.IndexError,
		"KeyError":          object.KeyError,
		"ZeroDivisionError": object.ZeroDivisionError,
		"AttributeError":    object.AttributeError,
		"NameError":         object.NameError,
		"ImportError":       object.ImportError,
		"Int":               object.IntConstructorFn,
		"Float":             object.FloatConstructorFn,
		"Str":               object.StrConstructorFn,
		"Bool":              object.BoolConstructorFn,
		"List":              object.ListConstructorFn,
		"Dict":              object.DictConstructorFn,
		"Chan":              object.ChanConstructorFn,
		"Function":          object.FunctionConstructorFn,
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
		fmt.Print(arg.String())
	}
	fmt.Print("\n")
	return nil, nil
}

// spawn launches a goblin function in a new goroutine, passing any extra
// positional arguments along to it. Goroutines are fire-and-forget: the
// function's return value and error are discarded, mirroring Go's `go`
// statement. Use a Chan to communicate results back.
func spawn(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("spawn", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return nil, object.NewTypeError("spawn() requires at least 1 argument")
	}
	fn, ok := args.Positional[0].(*object.Function)
	if !ok {
		return nil, object.NewTypeError("spawn() first argument must be a function, not %T", args.Positional[0])
	}
	callArgs := object.CallArgs{Positional: args.Positional[1:]}
	go fn.Call(callArgs)
	return object.Nil, nil
}

func range_(args object.CallArgs) (object.Object, error) {
	bound, err := object.BindArguments("range", []string{"start", "end"}, "", "", args)
	if err != nil {
		return nil, err
	}

	start, ok := bound["start"].(object.Integer)
	if !ok {
		return nil, object.NewTypeError("range() start argument must be an integer, got %T", bound["start"])
	}

	end, ok := bound["end"].(object.Integer)
	if !ok {
		return nil, object.NewTypeError("range() end argument must be an integer, got %T", bound["end"])
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
	if err := object.RequireNoKeyword("max", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return nil, object.NewTypeError("max() requires at least 1 argument")
	}

	var hasFloat bool
	for _, arg := range args.Positional {
		if _, ok := arg.(object.Float); ok {
			hasFloat = true
			break
		}
	}

	var maxValue float64
	if hasFloat {
		for i, arg := range args.Positional {
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
	for i, arg := range args.Positional {
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
	if err := object.RequireNoKeyword("min", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return nil, object.NewTypeError("min() requires at least 1 argument")
	}

	var hasFloat bool
	for _, arg := range args.Positional {
		if _, ok := arg.(object.Float); ok {
			hasFloat = true
			break
		}
	}

	var minValue float64
	if hasFloat {
		for i, arg := range args.Positional {
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
	for i, arg := range args.Positional {
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
