// Package math is the math module: Float functions, constants and basic
// statistics. Domain errors follow IEEE 754 and Go: they give NAN or an
// infinity rather than raising.
package math

import (
	"math"
	"sort"
	"strconv"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("math", map[string]object.Object{
		"PI":   object.Float(math.Pi),
		"E":    object.Float(math.E),
		"INF":  object.Float(math.Inf(1)),
		"NAN":  object.Float(math.NaN()),
		"sqrt": unary("sqrt", math.Sqrt),
		"exp":  unary("exp", math.Exp),
		"sin":  unary("sin", math.Sin),
		"cos":  unary("cos", math.Cos),
		"tan":  unary("tan", math.Tan),
		"asin": unary("asin", math.Asin),
		"acos": unary("acos", math.Acos),
		"atan": unary("atan", math.Atan),
	}, object.Funcs{
		"pow":        pow,
		"log":        log,
		"atan2":      atan2,
		"abs":        abs,
		"floor":      wholeNumber("floor", math.Floor),
		"ceil":       wholeNumber("ceil", math.Ceil),
		"round":      wholeNumber("round", math.Round),
		"is_nan":     isNaN,
		"mean":       mean,
		"median":     median,
		"stdev":      stdev,
		"percentile": percentile,
	}), nil
}

// unary wraps a float64 -> float64 function as a builtin taking one number x.
func unary(name string, fn func(float64) float64) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		p := object.NewArgParser(name, args)
		x := p.Float64("x")
		if err := p.Finish(); err != nil {
			return nil, err
		}
		return object.Float(fn(x)), nil
	}}
}

func pow(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("pow", args)
	x := p.Float64("x")
	y := p.Float64("y")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Pow(x, y)), nil
}

// log is the natural logarithm, or the logarithm in base. Bases 2 and 10 go
// through Log2 and Log10, which are exact for exact powers.
func log(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("log", args)
	x := p.Float64("x")
	baseArg, _ := p.OptionalAny("base")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if baseArg == nil || baseArg == object.Nil {
		return object.Float(math.Log(x)), nil
	}
	var base float64
	switch b := baseArg.(type) {
	case object.Integer:
		base = float64(b)
	case object.Float:
		base = float64(b)
	default:
		return nil, object.NewTypeError("log() argument 'base' must be Int, Float or Nil, got %s", baseArg.TypeName())
	}
	if !(base > 0) || base == 1 || math.IsInf(base, 1) {
		return nil, object.NewValueError("log() argument 'base' must be positive and not 1, got %s", formatFloat(base))
	}
	switch base {
	case 2:
		return object.Float(math.Log2(x)), nil
	case 10:
		return object.Float(math.Log10(x)), nil
	}
	return object.Float(math.Log(x) / math.Log(base)), nil
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func atan2(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("atan2", args)
	y := p.Float64("y")
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Float(math.Atan2(y, x)), nil
}

// abs keeps the kind of its argument: Int for an Int, Float for a Float.
func abs(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("abs", args)
	x := p.Number("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch n := x.(type) {
	case object.Integer:
		if n == math.MinInt64 {
			return nil, object.NewIntegerOverflow()
		}
		if n < 0 {
			return -n, nil
		}
		return n, nil
	case object.Float:
		return object.Float(math.Abs(float64(n))), nil
	}
	return nil, nil
}

// wholeNumber is floor, ceil or round: the result is an Int, because that
// is what it is used as. An Int argument is returned as it is.
func wholeNumber(name string, fn func(float64) float64) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		p := object.NewArgParser(name, args)
		x := p.Number("x")
		if err := p.Finish(); err != nil {
			return nil, err
		}
		n, ok := x.(object.Float)
		if !ok {
			return x, nil
		}
		f := float64(n)
		if math.IsNaN(f) {
			return nil, object.NewValueError("%s() cannot convert NaN", name)
		}
		if math.IsInf(f, 0) {
			return nil, object.NewValueError("%s() cannot convert %s", name, object.Float(f).String())
		}
		r := fn(f)
		// 2^63 is the first float64 past the int64 range.
		if r >= 9223372036854775808.0 || r < -9223372036854775808.0 {
			return nil, object.NewOverflowError("%s() value out of range: %s", name, object.Float(f).String())
		}
		return object.Integer(int64(r)), nil
	}
}

func isNaN(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("is_nan", args)
	x := p.Float64("x")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return object.Bool(math.IsNaN(x)), nil
}

// measurements reads the `values` argument of a statistics function: any
// iterable of Ints and Floats, read once, as float64s.
func measurements(name string, p *object.ArgParser) ([]float64, error) {
	values := p.Any("values")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return collect(name, values)
}

func collect(name string, values object.Object) ([]float64, error) {
	items, err := object.CollectArg(name, "values", values)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(items))
	for i, item := range items {
		switch n := item.(type) {
		case object.Integer:
			out[i] = float64(n)
		case object.Float:
			out[i] = float64(n)
		default:
			return nil, object.NewTypeError("%s() argument 'values' must contain only Int or Float, got %s", name, item.TypeName())
		}
	}
	return out, nil
}

func errEmpty(name string) error {
	return object.NewValueError("%s() argument 'values' is empty", name)
}

func hasNaN(values []float64) bool {
	for _, v := range values {
		if math.IsNaN(v) {
			return true
		}
	}
	return false
}

func average(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func mean(args object.CallArgs) (object.Object, error) {
	values, err := measurements("mean", object.NewArgParser("mean", args))
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errEmpty("mean")
	}
	return object.Float(average(values)), nil
}

// sorted is a sorted copy of values, which collect already made.
func sorted(values []float64) []float64 {
	sort.Float64s(values)
	return values
}

// rank is the value at fractional position r of sorted values,
// interpolating linearly between neighbours.
func rank(values []float64, r float64) float64 {
	lower := math.Floor(r)
	i := int(lower)
	if i+1 >= len(values) {
		return values[len(values)-1]
	}
	return values[i] + (r-lower)*(values[i+1]-values[i])
}

func median(args object.CallArgs) (object.Object, error) {
	values, err := measurements("median", object.NewArgParser("median", args))
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errEmpty("median")
	}
	if hasNaN(values) {
		return object.Float(math.NaN()), nil
	}
	return object.Float(rank(sorted(values), 0.5*float64(len(values)-1))), nil
}

// stdev is the sample standard deviation: two passes, divided by n - 1.
func stdev(args object.CallArgs) (object.Object, error) {
	values, err := measurements("stdev", object.NewArgParser("stdev", args))
	if err != nil {
		return nil, err
	}
	if len(values) < 2 {
		return nil, object.NewValueError("stdev() needs at least two values, got %d", len(values))
	}
	m := average(values)
	squares := 0.0
	for _, v := range values {
		squares += (v - m) * (v - m)
	}
	return object.Float(math.Sqrt(squares / float64(len(values)-1))), nil
}

func percentile(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("percentile", args)
	valuesArg := p.Any("values")
	pct := p.Float64("p")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if !(pct >= 0 && pct <= 100) {
		return nil, object.NewValueError("percentile() argument 'p' must be between 0 and 100, got %s", formatFloat(pct))
	}
	values, err := collect("percentile", valuesArg)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errEmpty("percentile")
	}
	if hasNaN(values) {
		return object.Float(math.NaN()), nil
	}
	return object.Float(rank(sorted(values), pct/100*float64(len(values)-1))), nil
}
