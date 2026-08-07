package extension

import (
	"math"

	"github.com/aisk/goblin/object"
)

func ExecuteMath() (object.Object, error) {
	return &object.Module{
		Name: "math",
		Members: map[string]object.Object{
			"pi":     object.Float(math.Pi),
			"e":      object.Float(math.E),
			"abs":    &object.Function{Name: "abs", Fn: mathAbs},
			"ceil":   &object.Function{Name: "ceil", Fn: mathCeil},
			"floor":  &object.Function{Name: "floor", Fn: mathFloor},
			"round":  &object.Function{Name: "round", Fn: mathRound},
			"pow":    &object.Function{Name: "pow", Fn: mathPow},
			"sqrt":   mathUnary("sqrt", math.Sqrt),
			"sin":    mathUnary("sin", math.Sin),
			"cos":    mathUnary("cos", math.Cos),
			"tan":    mathUnary("tan", math.Tan),
			"asin":   mathUnary("asin", math.Asin),
			"acos":   mathUnary("acos", math.Acos),
			"atan":   mathUnary("atan", math.Atan),
			"log":    mathUnary("log", math.Log),
			"log10":  mathUnary("log10", math.Log10),
			"exp":    mathUnary("exp", math.Exp),
			"max":    &object.Function{Name: "max", Fn: mathMax},
			"min":    &object.Function{Name: "min", Fn: mathMin},
			"is_nan": &object.Function{Name: "is_nan", Fn: mathIsNaN},
			"is_inf": &object.Function{Name: "is_inf", Fn: mathIsInf},
			"inf":    object.Float(math.Inf(1)),
			"nan":    object.Float(math.NaN()),
			"cbrt":   mathUnary("cbrt", math.Cbrt),
			"trunc":  &object.Function{Name: "trunc", Fn: mathTrunc},
			"log2":   mathUnary("log2", math.Log2),
			"sinh":   mathUnary("sinh", math.Sinh),
			"cosh":   mathUnary("cosh", math.Cosh),
			"tanh":   mathUnary("tanh", math.Tanh),
			"asinh":  mathUnary("asinh", math.Asinh),
			"acosh":  mathUnary("acosh", math.Acosh),
			"atanh":  mathUnary("atanh", math.Atanh),
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
	return nil, object.NewTypeError("%s() argument 'x' must be number, got %s", name, v.TypeName())
}

// mathUnary wraps a float64 -> float64 function from the math package as a
// builtin taking a single numeric argument x.
func mathUnary(name string, fn func(float64) float64) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		p := object.NewArgParser(name, args)
		x := p.Float64("x")
		if err := p.Finish(); err != nil {
			return nil, err
		}
		return object.Float(fn(x)), nil
	}}
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

func mathMax(args object.CallArgs) (object.Object, error) {
	return mathExtreme("max", args, func(candidate, best float64) bool { return candidate > best })
}

func mathMin(args object.CallArgs) (object.Object, error) {
	return mathExtreme("min", args, func(candidate, best float64) bool { return candidate < best })
}

// mathExtreme returns the winning argument itself, so Integer inputs stay
// Integer instead of being coerced to Float.
func mathExtreme(name string, args object.CallArgs, better func(candidate, best float64) bool) (object.Object, error) {
	p := object.NewArgParser(name, args)
	nums := p.Rest()
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(nums) < 2 {
		return nil, object.NewTypeError("%s() requires at least 2 arguments", name)
	}
	best := nums[0]
	bestVal, err := toFloat(name, best)
	if err != nil {
		return nil, err
	}
	for _, arg := range nums[1:] {
		f, err := toFloat(name, arg)
		if err != nil {
			return nil, err
		}
		if better(f, bestVal) {
			best, bestVal = arg, f
		}
	}
	return best, nil
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
		return 0, object.NewTypeError("%s() argument must be a number, got %s", funcName, v.TypeName())
	}
}
