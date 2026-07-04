package extension

import (
	"math"

	"github.com/aisk/goblin/object"
)

func ExecuteMath() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"pi":     object.Float(math.Pi),
			"e":      object.Float(math.E),
			"abs":    &object.Function{Name: "abs", Fn: mathAbs},
			"ceil":   &object.Function{Name: "ceil", Fn: mathCeil},
			"floor":  &object.Function{Name: "floor", Fn: mathFloor},
			"round":  &object.Function{Name: "round", Fn: mathRound},
			"pow":    &object.Function{Name: "pow", Fn: mathPow},
			"sqrt":   &object.Function{Name: "sqrt", Fn: mathSqrt},
			"sin":    &object.Function{Name: "sin", Fn: mathSin},
			"cos":    &object.Function{Name: "cos", Fn: mathCos},
			"tan":    &object.Function{Name: "tan", Fn: mathTan},
			"asin":   &object.Function{Name: "asin", Fn: mathAsin},
			"acos":   &object.Function{Name: "acos", Fn: mathAcos},
			"atan":   &object.Function{Name: "atan", Fn: mathAtan},
			"log":    &object.Function{Name: "log", Fn: mathLog},
			"log10":  &object.Function{Name: "log10", Fn: mathLog10},
			"exp":    &object.Function{Name: "exp", Fn: mathExp},
			"max":    &object.Function{Name: "max", Fn: mathMax},
			"min":    &object.Function{Name: "min", Fn: mathMin},
			"is_nan": &object.Function{Name: "is_nan", Fn: mathIsNaN},
			"is_inf": &object.Function{Name: "is_inf", Fn: mathIsInf},
			"inf":    object.Float(math.Inf(1)),
			"nan":    object.Float(math.NaN()),
			"cbrt":   &object.Function{Name: "cbrt", Fn: mathCbrt},
			"trunc":  &object.Function{Name: "trunc", Fn: mathTrunc},
			"log2":   &object.Function{Name: "log2", Fn: mathLog2},
			"sinh":   &object.Function{Name: "sinh", Fn: mathSinh},
			"cosh":   &object.Function{Name: "cosh", Fn: mathCosh},
			"tanh":   &object.Function{Name: "tanh", Fn: mathTanh},
			"asinh":  &object.Function{Name: "asinh", Fn: mathAsinh},
			"acosh":  &object.Function{Name: "acosh", Fn: mathAcosh},
			"atanh":  &object.Function{Name: "atanh", Fn: mathAtanh},
			"atan2":  &object.Function{Name: "atan2", Fn: mathAtan2},
			"hypot":  &object.Function{Name: "hypot", Fn: mathHypot},
		},
	}, nil
}

// mathIntPreserving dispatches a function that preserves the int-ness of its
// argument: an Integer input yields an Integer output via intFn, a Float input
// yields a Float output via floatFn.
func mathIntPreserving(name string, args object.CallArgs, intFn func(int64) int64, floatFn func(float64) float64) (object.Object, error) {
	p := object.NewArgParser(name, args)
	v := p.Number("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch n := v.(type) {
	case object.Integer:
		return object.Integer(intFn(int64(n))), nil
	case object.Float:
		return object.Float(floatFn(float64(n))), nil
	}
	return nil, p.Err()
}

func mathAbs(args object.CallArgs) (object.Object, error) {
	return mathIntPreserving("abs", args,
		func(i int64) int64 {
			if i < 0 {
				return -i
			}
			return i
		},
		math.Abs,
	)
}

func mathCeil(args object.CallArgs) (object.Object, error) {
	return mathIntPreserving("ceil", args,
		func(i int64) int64 { return i },
		math.Ceil,
	)
}

func mathFloor(args object.CallArgs) (object.Object, error) {
	return mathIntPreserving("floor", args,
		func(i int64) int64 { return i },
		math.Floor,
	)
}

func mathRound(args object.CallArgs) (object.Object, error) {
	return mathIntPreserving("round", args,
		func(i int64) int64 { return i },
		math.Round,
	)
}

func mathTrunc(args object.CallArgs) (object.Object, error) {
	return mathIntPreserving("trunc", args,
		func(i int64) int64 { return i },
		math.Trunc,
	)
}

func mathPow(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("pow", args)
	base := p.Float64("base")
	exp := p.Float64("exp")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Pow(base, exp)), nil
}

func mathSqrt(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("sqrt", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Sqrt(x)), nil
}

func mathSin(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("sin", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Sin(x)), nil
}

func mathCos(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("cos", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Cos(x)), nil
}

func mathTan(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("tan", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Tan(x)), nil
}

func mathAsin(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("asin", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Asin(x)), nil
}

func mathAcos(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("acos", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Acos(x)), nil
}

func mathAtan(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("atan", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Atan(x)), nil
}

func mathLog(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("log", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Log(x)), nil
}

func mathLog10(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("log10", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Log10(x)), nil
}

func mathExp(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("exp", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Exp(x)), nil
}

func mathMax(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("max", args)
	nums := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(nums) < 2 {
		return nil, object.NewTypeError("max() requires at least 2 arguments")
	}
	maxVal, err := toFloat("max", nums[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range nums[1:] {
		f, err := toFloat("max", arg)
		if err != nil {
			return nil, err
		}
		if f > maxVal {
			maxVal = f
		}
	}
	return object.Float(maxVal), nil
}

func mathMin(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("min", args)
	nums := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(nums) < 2 {
		return nil, object.NewTypeError("min() requires at least 2 arguments")
	}
	minVal, err := toFloat("min", nums[0])
	if err != nil {
		return nil, err
	}
	for _, arg := range nums[1:] {
		f, err := toFloat("min", arg)
		if err != nil {
			return nil, err
		}
		if f < minVal {
			minVal = f
		}
	}
	return object.Float(minVal), nil
}

func mathIsNaN(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("is_nan", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Bool(math.IsNaN(x)), nil
}

func mathIsInf(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("is_inf", args)
	x := p.Float64("x")
	dir := int(int64(p.IntOr("sign", 0)))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Bool(math.IsInf(x, dir)), nil
}

func mathCbrt(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("cbrt", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Cbrt(x)), nil
}

func mathLog2(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("log2", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Log2(x)), nil
}

func mathSinh(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("sinh", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Sinh(x)), nil
}

func mathCosh(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("cosh", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Cosh(x)), nil
}

func mathTanh(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("tanh", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Tanh(x)), nil
}

func mathAsinh(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("asinh", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Asinh(x)), nil
}

func mathAcosh(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("acosh", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Acosh(x)), nil
}

func mathAtanh(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("atanh", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Atanh(x)), nil
}

func mathAtan2(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("atan2", args)
	y := p.Float64("y")
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Atan2(y, x)), nil
}

func mathHypot(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("hypot", args)
	pv := p.Float64("p")
	qv := p.Float64("q")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Hypot(pv, qv)), nil
}

// toFloat coerces an Integer or Float argument to float64. It is used for the
// variadic math functions (max, min) where each element supplied via Rest must
// be validated individually; single-argument functions use ArgParser.Float64
// instead.
func toFloat(funcName string, v object.Object) (float64, error) {
	switch n := v.(type) {
	case object.Integer:
		return float64(int64(n)), nil
	case object.Float:
		return float64(n), nil
	default:
		return 0, object.NewTypeError("%s() argument must be a number, got %T", funcName, v)
	}
}
