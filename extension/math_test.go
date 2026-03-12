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
	tests := []struct {
		name    string
		args    object.Args
		wantErr string
	}{
		{name: "sqrt", args: object.Args{object.String("x")}, wantErr: "sqrt() argument must be a number, got object.String"},
		{name: "pow", args: object.Args{object.Integer(2), object.String("x")}, wantErr: "pow() argument must be a number, got object.String"},
		{name: "max", args: object.Args{object.Integer(1), object.String("x")}, wantErr: "max() argument must be a number, got object.String"},
		{name: "min", args: object.Args{object.String("x"), object.Integer(1)}, wantErr: "min() argument must be a number, got object.String"},
		{name: "is_nan", args: object.Args{object.Bool(true)}, wantErr: "is_nan() argument must be a number, got object.Bool"},
		{name: "is_inf", args: object.Args{object.Bool(true)}, wantErr: "is_inf() argument must be a number, got object.Bool"},
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
