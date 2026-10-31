package object

import (
	"errors"
	"math"
	"testing"
)

func TestCheckedIntegerArithmetic(t *testing.T) {
	const max, min = math.MaxInt64, math.MinInt64
	cases := []struct {
		name string
		got  func() (int64, bool)
		want int64
		ok   bool
	}{
		{"add", func() (int64, bool) { return CheckedAdd(max-1, 1) }, max, true},
		{"add overflow", func() (int64, bool) { return CheckedAdd(max, 1) }, 0, false},
		{"add underflow", func() (int64, bool) { return CheckedAdd(min, -1) }, 0, false},
		{"sub", func() (int64, bool) { return CheckedSub(min+1, 1) }, min, true},
		{"sub overflow", func() (int64, bool) { return CheckedSub(min, 1) }, 0, false},
		{"sub of min", func() (int64, bool) { return CheckedSub(0, min) }, 0, false},
		{"mul small", func() (int64, bool) { return CheckedMul(-46341, 46341) }, -2147488281, true},
		{"mul large", func() (int64, bool) { return CheckedMul(3037000499, 3037000499) }, 9223372030926249001, true},
		{"mul overflow", func() (int64, bool) { return CheckedMul(3037000500, 3037000500) }, 0, false},
		{"mul min by -1", func() (int64, bool) { return CheckedMul(min, -1) }, 0, false},
		{"mul -1 by min", func() (int64, bool) { return CheckedMul(-1, min) }, 0, false},
		{"mul min by 1", func() (int64, bool) { return CheckedMul(min, 1) }, min, true},
		{"div min by -1", func() (int64, bool) { return CheckedDiv(min, -1) }, 0, false},
		{"div", func() (int64, bool) { return CheckedDiv(-7, 2) }, -3, true},
		{"neg min", func() (int64, bool) { return CheckedNeg(min) }, 0, false},
		{"neg", func() (int64, bool) { return CheckedNeg(max) }, -max, true},
	}
	for _, tc := range cases {
		got, ok := tc.got()
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("%s = %d, %v; want %d, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestIntegerOverflowRaises(t *testing.T) {
	ops := map[string]func() (Object, error){
		"add":    func() (Object, error) { return Add(Integer(math.MaxInt64), Integer(1)) },
		"addInt": func() (Object, error) { return AddInt(Integer(math.MaxInt64), 1) },
		"mul":    func() (Object, error) { return Multiply(Integer(math.MinInt64), Integer(-1)) },
		"div":    func() (Object, error) { return Divide(Integer(math.MinInt64), Integer(-1)) },
		"neg":    func() (Object, error) { return Negate(Integer(math.MinInt64)) },
		"repeat": func() (Object, error) { return String("ab").Multiply(Integer(math.MaxInt64)) },
	}
	for name, op := range ops {
		if _, err := op(); !errors.Is(err, OverflowError) {
			t.Errorf("%s: err = %v, want OverflowError", name, err)
		}
	}
}

func TestFloatAlwaysLooksLikeAFloat(t *testing.T) {
	// A variable, not a constant: Go folds 0.1 + 0.2 to exactly 0.3.
	tenth := 0.1
	cases := map[float64]string{
		2:                           "2.0",
		-0.5:                        "-0.5",
		0:                           "0.0",
		tenth + 0.2:                 "0.30000000000000004",
		1e15:                        "1000000000000000.0",
		1e16:                        "1e+16",
		0.0001:                      "0.0001",
		0.00001:                     "1e-05",
		math.Inf(-1):                "-Inf",
		123456789.125:               "123456789.125",
		math.MaxFloat64:             "1.7976931348623157e+308",
		math.SmallestNonzeroFloat64: "5e-324",
	}
	for v, want := range cases {
		if got := Float(v).String(); got != want {
			t.Errorf("Float(%v).String() = %q, want %q", v, got, want)
		}
	}
}
