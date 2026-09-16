package object

import (
	"fmt"
	"testing"
)

// The native-right variants must answer exactly what the boxed entry points
// answer, for numbers, mixed numbers, and anything else.
func TestNativeRightOperatorsMatchBoxed(t *testing.T) {
	lefts := []Object{Integer(7), Float(7.5), String("s"), Nil, &List{}}
	rights := []int64{3, 0, -2, 1000}

	type binary struct {
		name   string
		native func(Object, int64) (Object, error)
		boxed  func(Object, Object) (Object, error)
	}
	ops := []binary{
		{"Add", AddInt, Add}, {"Minus", MinusInt, Minus}, {"Multiply", MultiplyInt, Multiply},
		{"Divide", DivideInt, Divide}, {"Modulo", ModuloInt, Modulo},
	}
	for _, op := range ops {
		for _, a := range lefts {
			for _, b := range rights {
				got, gotErr := op.native(a, b)
				want, wantErr := op.boxed(a, Integer(b))
				if (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
					t.Fatalf("%sInt(%v, %d): error %v, boxed %v", op.name, a, b, gotErr, wantErr)
				}
				if gotErr == nil && fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("%sInt(%v, %d) = %v, boxed %v", op.name, a, b, got, want)
				}
			}
		}
	}

	for _, a := range lefts {
		for _, b := range rights {
			orders := []struct {
				name   string
				native func(Object, int64) (bool, error)
				boxed  func(Object, Object) (bool, error)
			}{
				{"Less", LessInt, Less}, {"LessEqual", LessEqualInt, LessEqual},
				{"Greater", GreaterInt, Greater}, {"GreaterEqual", GreaterEqualInt, GreaterEqual},
				{"NotEquals", NotEqualsInt, NotEquals},
			}
			for _, op := range orders {
				got, gotErr := op.native(a, b)
				want, wantErr := op.boxed(a, Integer(b))
				if (gotErr == nil) != (wantErr == nil) || got != want || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
					t.Fatalf("%sInt(%v, %d) = %v, %v; boxed %v, %v", op.name, a, b, got, gotErr, want, wantErr)
				}
			}
			eq, err := EqualsInt(a, b)
			wantEq, wantEqErr := Equals(a, Integer(b))
			if err != nil || wantEqErr != nil || eq != wantEq {
				t.Fatalf("EqualsInt(%v, %d) = %v, %v; boxed %v, %v", a, b, eq, err, wantEq, wantEqErr)
			}
		}
	}
}
