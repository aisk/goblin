package extension

import (
	"fmt"

	"github.com/aisk/goblin/object"
)

var BuiltinsModule = &object.Module{
	Members: map[string]object.Object{
		"print": &object.Function{Name: "print", Fn: print},
		"range": &object.Function{Name: "range", Fn: range_},
		"max":   &object.Function{Name: "max", Fn: max},
		"min":   &object.Function{Name: "min", Fn: min},
	},
}

func print(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	for i, arg := range args {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(arg.String())
	}
	fmt.Print("\n")
	return nil, nil
}

func range_(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("range() takes exactly 2 arguments, got %d", len(args))
	}

	start, ok := args[0].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("range() first argument must be an integer, got %T", args[0])
	}

	end, ok := args[1].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("range() second argument must be an integer, got %T", args[1])
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

func max(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("max() requires at least 1 argument")
	}

	var hasFloat bool
	for _, arg := range args {
		if _, ok := arg.(object.Float); ok {
			hasFloat = true
			break
		}
	}

	var maxValue float64
	if hasFloat {
		for i, arg := range args {
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
				return nil, fmt.Errorf("max() argument %d: invalid type %T", i, arg)
			}
		}
		return object.Float(maxValue), nil
	}

	maxIntValue := int64(0)
	for i, arg := range args {
		if v, ok := arg.(object.Integer); ok {
			if i == 0 || int64(v) > maxIntValue {
				maxIntValue = int64(v)
			}
		} else {
			return nil, fmt.Errorf("max() argument %d: invalid type %T", i, arg)
		}
	}
	return object.Integer(maxIntValue), nil
}

func min(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("min() requires at least 1 argument")
	}

	var hasFloat bool
	for _, arg := range args {
		if _, ok := arg.(object.Float); ok {
			hasFloat = true
			break
		}
	}

	var minValue float64
	if hasFloat {
		for i, arg := range args {
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
				return nil, fmt.Errorf("min() argument %d: invalid type %T", i, arg)
			}
		}
		return object.Float(minValue), nil
	}

	minIntValue := int64(0)
	for i, arg := range args {
		if v, ok := arg.(object.Integer); ok {
			if i == 0 || int64(v) < minIntValue {
				minIntValue = int64(v)
			}
		} else {
			return nil, fmt.Errorf("min() argument %d: invalid type %T", i, arg)
		}
	}
	return object.Integer(minIntValue), nil
}
