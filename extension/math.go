package extension

import (
	"fmt"
	"math"

	"github.com/aisk/goblin/object"
)

func ExecuteMath() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"pi":      object.Float(math.Pi),
			"e":       object.Float(math.E),
			"abs":     &object.Function{Name: "abs", Fn: mathAbs},
			"ceil":    &object.Function{Name: "ceil", Fn: mathCeil},
			"floor":   &object.Function{Name: "floor", Fn: mathFloor},
			"round":   &object.Function{Name: "round", Fn: mathRound},
			"pow":     &object.Function{Name: "pow", Fn: mathPow},
			"sqrt":    &object.Function{Name: "sqrt", Fn: mathSqrt},
			"sin":     &object.Function{Name: "sin", Fn: mathSin},
			"cos":     &object.Function{Name: "cos", Fn: mathCos},
			"tan":     &object.Function{Name: "tan", Fn: mathTan},
			"asin":    &object.Function{Name: "asin", Fn: mathAsin},
			"acos":    &object.Function{Name: "acos", Fn: mathAcos},
			"atan":    &object.Function{Name: "atan", Fn: mathAtan},
			"log":     &object.Function{Name: "log", Fn: mathLog},
			"log10":   &object.Function{Name: "log10", Fn: mathLog10},
			"exp":     &object.Function{Name: "exp", Fn: mathExp},
			"max":     &object.Function{Name: "max", Fn: mathMax},
			"min":     &object.Function{Name: "min", Fn: mathMin},
			"is_nan":  &object.Function{Name: "is_nan", Fn: mathIsNaN},
			"is_inf":  &object.Function{Name: "is_inf", Fn: mathIsInf},
		},
	}, nil
}

func mathAbs(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("abs", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("abs() requires exactly 1 argument")
	}
	switch v := args.Positional[0].(type) {
	case object.Integer:
		if v < 0 {
			return object.Integer(-int64(v)), nil
		}
		return v, nil
	case object.Float:
		return object.Float(math.Abs(float64(v))), nil
	default:
		return nil, fmt.Errorf("abs() argument must be a number, got %T", args.Positional[0])
	}
}

func mathCeil(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("ceil", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("ceil() requires exactly 1 argument")
	}
	switch v := args.Positional[0].(type) {
	case object.Integer:
		return v, nil
	case object.Float:
		return object.Float(math.Ceil(float64(v))), nil
	default:
		return nil, fmt.Errorf("ceil() argument must be a number, got %T", args.Positional[0])
	}
}

func mathFloor(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("floor", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("floor() requires exactly 1 argument")
	}
	switch v := args.Positional[0].(type) {
	case object.Integer:
		return v, nil
	case object.Float:
		return object.Float(math.Floor(float64(v))), nil
	default:
		return nil, fmt.Errorf("floor() argument must be a number, got %T", args.Positional[0])
	}
}

func mathRound(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("round", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("round() requires exactly 1 argument")
	}
	switch v := args.Positional[0].(type) {
	case object.Integer:
		return v, nil
	case object.Float:
		return object.Float(math.Round(float64(v))), nil
	default:
		return nil, fmt.Errorf("round() argument must be a number, got %T", args.Positional[0])
	}
}

func mathPow(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("pow", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 2 {
		return nil, fmt.Errorf("pow() requires exactly 2 arguments")
	}
	base, ok1 := args.Positional[0].(object.Object)
	exp, ok2 := args.Positional[1].(object.Object)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("pow() arguments must be numbers")
	}
	baseFloat := toFloat(base)
	expFloat := toFloat(exp)
	return object.Float(math.Pow(baseFloat, expFloat)), nil
}

func mathSqrt(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("sqrt", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("sqrt() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Sqrt(f)), nil
}

func mathSin(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("sin", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("sin() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Sin(f)), nil
}

func mathCos(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("cos", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("cos() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Cos(f)), nil
}

func mathTan(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("tan", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("tan() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Tan(f)), nil
}

func mathAsin(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("asin", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("asin() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Asin(f)), nil
}

func mathAcos(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("acos", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("acos() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Acos(f)), nil
}

func mathAtan(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("atan", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("atan() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Atan(f)), nil
}

func mathLog(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("log", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("log() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Log(f)), nil
}

func mathLog10(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("log10", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("log10() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Log10(f)), nil
}

func mathExp(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("exp", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("exp() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Float(math.Exp(f)), nil
}

func mathMax(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("max", args); err != nil {
		return nil, err
	}
	if len(args.Positional) < 2 {
		return nil, fmt.Errorf("max() requires at least 2 arguments")
	}
	maxVal := toFloat(args.Positional[0])
	for _, arg := range args.Positional[1:] {
		f := toFloat(arg)
		if f > maxVal {
			maxVal = f
		}
	}
	return object.Float(maxVal), nil
}

func mathMin(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("min", args); err != nil {
		return nil, err
	}
	if len(args.Positional) < 2 {
		return nil, fmt.Errorf("min() requires at least 2 arguments")
	}
	minVal := toFloat(args.Positional[0])
	for _, arg := range args.Positional[1:] {
		f := toFloat(arg)
		if f < minVal {
			minVal = f
		}
	}
	return object.Float(minVal), nil
}

func mathIsNaN(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("is_nan", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("is_nan() requires exactly 1 argument")
	}
	f := toFloat(args.Positional[0])
	return object.Bool(math.IsNaN(f)), nil
}

func mathIsInf(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("is_inf", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 && len(args.Positional) != 2 {
		return nil, fmt.Errorf("is_inf() requires 1 or 2 arguments")
	}
	f := toFloat(args.Positional[0])
	var dir int = 0
	if len(args.Positional) == 2 {
		dirInt, ok := args.Positional[1].(object.Integer)
		if !ok {
			return nil, fmt.Errorf("is_inf() second argument must be an integer")
		}
		dir = int(dirInt)
	}
	return object.Bool(math.IsInf(f, dir)), nil
}

func toFloat(v object.Object) float64 {
	switch n := v.(type) {
	case object.Integer:
		return float64(int64(n))
	case object.Float:
		return float64(n)
	default:
		return 0
	}
}
