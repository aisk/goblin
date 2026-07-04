package extension

import (
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func mathFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := ExecuteMath()
	if err != nil {
		t.Fatalf("ExecuteMath() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("ExecuteMath() returned %T", modObj)
	}

	member, ok := mod.Members[name]
	if !ok {
		t.Fatalf("math module missing %q", name)
	}

	fn, ok := member.(*object.Function)
	if !ok {
		t.Fatalf("math module member %q is %T", name, member)
	}

	return fn
}

func TestMathFunctionsRejectNonNumericArgs(t *testing.T) {
	// Single-argument functions go through ArgParser.Number/Float64, which
	// reports "argument '<name>' must be number"; the variadic max/min stay on
	// the legacy toFloat helper, which reports "argument must be a number".
	tests := []struct {
		name    string
		args    object.Args
		wantErr string
	}{
		{name: "sqrt", args: object.Args{object.String("x")}, wantErr: "sqrt() argument 'x' must be number, got object.String"},
		{name: "pow", args: object.Args{object.Integer(2), object.String("x")}, wantErr: "pow() argument 'exp' must be number, got object.String"},
		{name: "max", args: object.Args{object.Integer(1), object.String("x")}, wantErr: "max() argument must be a number, got object.String"},
		{name: "min", args: object.Args{object.String("x"), object.Integer(1)}, wantErr: "min() argument must be a number, got object.String"},
		{name: "is_nan", args: object.Args{object.Bool(true)}, wantErr: "is_nan() argument 'x' must be number, got object.Bool"},
		{name: "is_inf", args: object.Args{object.Bool(true)}, wantErr: "is_inf() argument 'x' must be number, got object.Bool"},
		{name: "cbrt", args: object.Args{object.String("x")}, wantErr: "cbrt() argument 'x' must be number, got object.String"},
		{name: "trunc", args: object.Args{object.String("x")}, wantErr: "trunc() argument 'x' must be number, got object.String"},
		{name: "log2", args: object.Args{object.String("x")}, wantErr: "log2() argument 'x' must be number, got object.String"},
		{name: "sinh", args: object.Args{object.String("x")}, wantErr: "sinh() argument 'x' must be number, got object.String"},
		{name: "cosh", args: object.Args{object.String("x")}, wantErr: "cosh() argument 'x' must be number, got object.String"},
		{name: "tanh", args: object.Args{object.String("x")}, wantErr: "tanh() argument 'x' must be number, got object.String"},
		{name: "asinh", args: object.Args{object.String("x")}, wantErr: "asinh() argument 'x' must be number, got object.String"},
		{name: "acosh", args: object.Args{object.String("x")}, wantErr: "acosh() argument 'x' must be number, got object.String"},
		{name: "atanh", args: object.Args{object.String("x")}, wantErr: "atanh() argument 'x' must be number, got object.String"},
		{name: "atan2", args: object.Args{object.Integer(1), object.String("x")}, wantErr: "atan2() argument 'x' must be number, got object.String"},
		{name: "hypot", args: object.Args{object.String("x"), object.Integer(1)}, wantErr: "hypot() argument 'p' must be number, got object.String"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mathFunction(t, tt.name).Call(object.CallArgs{Positional: tt.args})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
