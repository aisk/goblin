package math

import (
	"errors"
	"math"
	"sort"
	"testing"

	"github.com/aisk/goblin/object"
)

func member(name string) object.Object {
	m, _ := Execute()
	return m.(*object.Module).Members[name]
}

func call(name string, args ...object.Object) (object.Object, error) {
	return member(name).(*object.Function).Call(object.CallArgs{Positional: args})
}

func callKw(name string, args object.Args, kws ...object.KwArg) (object.Object, error) {
	return member(name).(*object.Function).Call(object.CallArgs{Positional: args, Keyword: kws})
}

func list(values ...object.Object) *object.List { return &object.List{Elements: values} }

func floats(values ...float64) *object.List {
	l := list()
	for _, v := range values {
		l.Elements = append(l.Elements, object.Float(v))
	}
	return l
}

func TestMembers(t *testing.T) {
	m, _ := Execute()
	var names []string
	for name := range m.(*object.Module).Members {
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{"E", "INF", "NAN", "PI", "abs", "acos", "asin", "atan", "atan2", "ceil", "cos", "exp", "floor", "is_nan", "log", "mean", "median", "percentile", "pow", "round", "sin", "sqrt", "stdev", "tan"}
	if len(names) != len(want) {
		t.Fatalf("members = %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("members = %v", names)
		}
	}
	if member("PI") != object.Float(math.Pi) || member("INF") != object.Float(math.Inf(1)) || !math.IsNaN(float64(member("NAN").(object.Float))) {
		t.Fatal("constants")
	}
}

func TestWholeNumbers(t *testing.T) {
	cases := []struct {
		name string
		in   object.Object
		want object.Object
	}{
		{"floor", object.Float(2.7), object.Integer(2)},
		{"floor", object.Float(-2.5), object.Integer(-3)},
		{"ceil", object.Float(2.1), object.Integer(3)},
		{"ceil", object.Float(-2.9), object.Integer(-2)},
		{"round", object.Float(2.5), object.Integer(3)},
		{"round", object.Float(-2.5), object.Integer(-3)},
		{"round", object.Float(2.4), object.Integer(2)},
		{"round", object.Integer(7), object.Integer(7)},
		{"floor", object.Integer(-7), object.Integer(-7)},
		{"round", object.Float(-9223372036854775808), object.Integer(math.MinInt64)},
	}
	for _, c := range cases {
		got, err := call(c.name, c.in)
		if err != nil || got != c.want {
			t.Errorf("%s(%v) = %v (%T), %v", c.name, c.in, got, got, err)
		}
	}
	for _, c := range []struct {
		in   float64
		kind *object.Error
		msg  string
	}{
		{math.NaN(), object.ValueError, "floor() cannot convert NaN"},
		{math.Inf(1), object.ValueError, "floor() cannot convert +Inf"},
		{math.Inf(-1), object.ValueError, "floor() cannot convert -Inf"},
		{1e19, object.OverflowError, "floor() value out of range: 1e+19"},
		{9223372036854775807, object.OverflowError, "floor() value out of range: 9.223372036854776e+18"},
	} {
		_, err := call("floor", object.Float(c.in))
		if !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("floor(%v) = %v", c.in, err)
		}
	}
}

func TestFunctions(t *testing.T) {
	check := func(got object.Object, err error, want float64) {
		t.Helper()
		f, ok := got.(object.Float)
		if err != nil || !ok || (float64(f) != want && !(math.IsNaN(want) && math.IsNaN(float64(f)))) {
			t.Errorf("got %v (%T), %v; want %v", got, got, err, want)
		}
	}
	got, err := call("sqrt", object.Integer(16))
	check(got, err, 4)
	got, err = call("sqrt", object.Integer(-1))
	check(got, err, math.NaN())
	got, err = call("pow", object.Integer(2), object.Integer(10))
	check(got, err, 1024)
	got, err = callKw("pow", nil, object.KwArg{Name: "x", Value: object.Integer(2)}, object.KwArg{Name: "y", Value: object.Float(0.5)})
	check(got, err, math.Sqrt2)
	got, err = call("log", object.Float(math.E))
	check(got, err, 1)
	got, err = callKw("log", object.Args{object.Integer(8)}, object.KwArg{Name: "base", Value: object.Integer(2)})
	check(got, err, 3)
	got, err = callKw("log", object.Args{object.Integer(1000)}, object.KwArg{Name: "base", Value: object.Integer(10)})
	check(got, err, 3)
	got, err = callKw("log", object.Args{object.Integer(81)}, object.KwArg{Name: "base", Value: object.Float(3)})
	check(got, err, math.Log(81)/math.Log(3))
	got, err = callKw("log", object.Args{object.Integer(5)}, object.KwArg{Name: "base", Value: object.Nil})
	check(got, err, math.Log(5))
	got, err = call("log", object.Integer(0))
	check(got, err, math.Inf(-1))
	got, err = call("log", object.Integer(-1))
	check(got, err, math.NaN())
	for _, base := range []object.Object{object.Integer(1), object.Integer(0), object.Float(-2), object.Float(math.NaN())} {
		_, err := callKw("log", object.Args{object.Integer(8)}, object.KwArg{Name: "base", Value: base})
		if !errors.Is(err, object.ValueError) {
			t.Errorf("log base %v = %v", base, err)
		}
	}
	got, err = call("atan2", object.Integer(1), object.Integer(-1))
	check(got, err, 3*math.Pi/4)
	got, err = call("exp", object.Integer(0))
	check(got, err, 1)
	if got, _ := call("abs", object.Integer(-3)); got != object.Integer(3) {
		t.Errorf("abs(-3) = %v", got)
	}
	if got, _ := call("abs", object.Float(-2.5)); got != object.Float(2.5) {
		t.Errorf("abs(-2.5) = %v", got)
	}
	if _, err := call("abs", object.Integer(math.MinInt64)); !errors.Is(err, object.OverflowError) {
		t.Errorf("abs(min Int) = %v", err)
	}
	if got, _ := call("is_nan", member("NAN")); got != object.True {
		t.Error("is_nan(NAN)")
	}
	_, err = call("sqrt", object.String("4"))
	if !errors.Is(err, object.TypeError) || err.Error() != "sqrt() argument 'x' must be Int or Float, got String" {
		t.Errorf("sqrt(String) = %v", err)
	}
}

func TestStatistics(t *testing.T) {
	timings := floats(0.82, 0.79, 1.31, 0.80, 0.85)
	near := func(name string, got object.Object, err error, want float64) {
		t.Helper()
		f, ok := got.(object.Float)
		if err != nil || !ok || math.Abs(float64(f)-want) > 1e-12 {
			t.Errorf("%s = %v (%T), %v; want %v", name, got, got, err, want)
		}
	}
	got, err := call("mean", timings)
	near("mean", got, err, 0.914)
	got, err = call("median", timings)
	near("median", got, err, 0.82)
	got, err = call("median", floats(4, 1, 3, 2))
	near("even median", got, err, 2.5)
	got, err = call("stdev", timings)
	near("stdev", got, err, 0.2225533643870611)
	got, err = call("percentile", timings, object.Integer(95))
	near("p95", got, err, 1.218)
	got, err = call("percentile", timings, object.Integer(0))
	near("p0", got, err, 0.79)
	got, err = call("percentile", timings, object.Integer(100))
	near("p100", got, err, 1.31)
	got, err = call("percentile", floats(1, 2, 3, 4), object.Float(50))
	near("p50", got, err, 2.5)
	// Always a Float, whatever the inputs; any iterable; the input is not changed.
	ints := list(object.Integer(3), object.Integer(1), object.Integer(2))
	if got, _ := call("mean", ints); got != object.Float(2) {
		t.Errorf("mean of Ints = %v (%T)", got, got)
	}
	if got, _ := call("median", ints); got != object.Float(2) || ints.Elements[0] != object.Integer(3) {
		t.Errorf("median changed its input or the type: %v", got)
	}
	d := object.NewDict()
	d.Set(object.String("a"), object.Integer(10))
	d.Set(object.String("b"), object.Integer(20))
	values, _ := object.CallMethod(d, "values", object.CallArgs{})
	if got, _ := call("mean", values); got != object.Float(15) {
		t.Errorf("mean of Dict values = %v", got)
	}
	got, _ = call("median", floats(1, math.NaN(), 3))
	if !math.IsNaN(float64(got.(object.Float))) {
		t.Errorf("NAN in median = %v", got)
	}
	got, _ = call("percentile", floats(1, math.NaN()), object.Integer(0))
	if !math.IsNaN(float64(got.(object.Float))) {
		t.Errorf("NAN in percentile = %v", got)
	}
	// An infinity is an ordinary value: a rank on or next to it is not NaN.
	inf := math.Inf(1)
	for _, c := range []struct {
		name string
		args object.Args
		want float64
	}{
		{"median", object.Args{floats(1, 2, inf)}, 2},
		{"median", object.Args{floats(1, inf)}, inf},
		{"median", object.Args{floats(1, inf, inf)}, inf},
		{"median", object.Args{floats(inf, inf)}, inf},
		{"percentile", object.Args{floats(1, 2, inf), object.Integer(50)}, 2},
		{"percentile", object.Args{floats(1, inf), object.Integer(0)}, 1},
		{"percentile", object.Args{floats(-inf, 1, 2), object.Integer(0)}, -inf},
		{"percentile", object.Args{floats(-inf, 1, 2), object.Integer(100)}, 2},
	} {
		got, err := call(c.name, c.args...)
		if err != nil || got != object.Float(c.want) {
			t.Errorf("%s%v = %v, %v; want %v", c.name, c.args, got, err, c.want)
		}
	}
	for _, c := range []struct {
		name string
		args object.Args
		kind *object.Error
		msg  string
	}{
		{"mean", object.Args{list()}, object.ValueError, "mean() argument 'values' is empty"},
		{"median", object.Args{list()}, object.ValueError, "median() argument 'values' is empty"},
		{"percentile", object.Args{list(), object.Integer(5)}, object.ValueError, "percentile() argument 'values' is empty"},
		{"stdev", object.Args{floats(1)}, object.ValueError, "stdev() needs at least two values, got 1"},
		{"stdev", object.Args{list()}, object.ValueError, "stdev() needs at least two values, got 0"},
		{"percentile", object.Args{floats(1), object.Integer(101)}, object.ValueError, "percentile() argument 'p' must be between 0 and 100, got 101"},
		{"percentile", object.Args{floats(1), object.Float(-0.5)}, object.ValueError, "percentile() argument 'p' must be between 0 and 100, got -0.5"},
		{"mean", object.Args{list(object.Integer(1), object.String("2"))}, object.TypeError, "mean() argument 'values' must contain only Int or Float, got String"},
		{"mean", object.Args{list(object.True)}, object.TypeError, "mean() argument 'values' must contain only Int or Float, got Bool"},
		{"median", object.Args{object.Integer(3)}, object.TypeError, "median() argument 'values' must be iterable, got Int"},
	} {
		_, err := call(c.name, c.args...)
		if !errors.Is(err, c.kind) || err.Error() != c.msg {
			t.Errorf("%s(%v) = %v, want %s", c.name, c.args, err, c.msg)
		}
	}
}
