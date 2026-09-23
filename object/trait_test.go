package object

import (
	"errors"
	"strings"
	"testing"
)

// fakeValue is a minimal UserValue, standing in for the interpreter's instances
// and the transpiler's generated structs.
type fakeValue struct {
	NoAssignment
	typ    *UserType
	fields []Object
}

func (v *fakeValue) UserType() *UserType   { return v.typ }
func (v *fakeValue) FieldValues() []Object { return v.fields }
func (v *fakeValue) TypeName() string      { return v.typ.Name }
func (v *fakeValue) String() string        { return UserString(v) }
func (v *fakeValue) ToString() (string, error) {
	return UserToString(v)
}
func (v *fakeValue) ToBool() (bool, error)             { return UserToBool(v) }
func (v *fakeValue) Hash() (uint64, error)             { return UserHash(v) }
func (v *fakeValue) Equals(o Object) (bool, error)     { return UserEquals(v, o) }
func (v *fakeValue) Compare(o Object) (int, error)     { return UserCompare(v, o) }
func (v *fakeValue) Add(o Object) (Object, error)      { return UserArith(v, ArithAdd, o) }
func (v *fakeValue) Minus(o Object) (Object, error)    { return UserArith(v, ArithSub, o) }
func (v *fakeValue) Multiply(o Object) (Object, error) { return UserArith(v, ArithMul, o) }
func (v *fakeValue) Divide(o Object) (Object, error)   { return UserArith(v, ArithDiv, o) }
func (v *fakeValue) Modulo(o Object) (Object, error)   { return UserArith(v, ArithMod, o) }
func (v *fakeValue) RAdd(o Object) (Object, bool, error) {
	return UserReflected(v, ArithAdd, o)
}
func (v *fakeValue) RMinus(o Object) (Object, bool, error) {
	return UserReflected(v, ArithSub, o)
}
func (v *fakeValue) RMultiply(o Object) (Object, bool, error) {
	return UserReflected(v, ArithMul, o)
}
func (v *fakeValue) RDivide(o Object) (Object, bool, error) {
	return UserReflected(v, ArithDiv, o)
}
func (v *fakeValue) RModulo(o Object) (Object, bool, error) {
	return UserReflected(v, ArithMod, o)
}
func (v *fakeValue) Iter() ([]Object, error)            { return UserIter(v) }
func (v *fakeValue) Index(i Object) (Object, error)     { return UserIndex(v, i) }
func (v *fakeValue) GetAttr(string) (Object, error)     { return nil, NewAttributeError("none") }
func (v *fakeValue) Attributes() []string               { return nil }
func (v *fakeValue) SetIndex(i, x Object) (bool, error) { return UserSetIndex(v, i, x) }

// sealed builds a type from impls, failing the test when they do not validate.
func sealed(t *testing.T, name string, fields []string, impls ...ImplSpec) *UserType {
	t.Helper()
	typ := NewUserType(name, fields)
	for _, impl := range impls {
		typ.Implement(impl.Trait, impl.Methods)
	}
	if err := typ.Seal(); err != nil {
		t.Fatal(err)
	}
	return typ
}

func method(name string, arity int, fn func(args []Object) (Object, error)) ImplMethod {
	return ImplMethod{Name: name, Arity: arity, Fn: goFn(name, fn)}
}

func TestStructuralImpls(t *testing.T) {
	point := sealed(t, "Point", []string{"x", "y"},
		ImplSpec{Trait: EqTrait}, ImplSpec{Trait: OrdTrait}, ImplSpec{Trait: HashableTrait}, ImplSpec{Trait: ShowTrait})
	p := func(x, y Object) *fakeValue { return &fakeValue{typ: point, fields: []Object{x, y}} }

	if eq, err := Equals(p(Integer(1), String("a")), p(Float(1), String("a"))); err != nil || !eq {
		t.Fatalf("structural eq = %v, %v", eq, err)
	}
	other := sealed(t, "Other", []string{"x", "y"}, ImplSpec{Trait: EqTrait})
	if eq, _ := Equals(p(Integer(1), Integer(2)), &fakeValue{typ: other, fields: []Object{Integer(1), Integer(2)}}); eq {
		t.Fatal("values of different types must be unequal")
	}
	if lt, err := Less(p(Integer(1), Integer(9)), p(Integer(2), Integer(0))); err != nil || !lt {
		t.Fatalf("lexicographic compare = %v, %v", lt, err)
	}
	if _, err := Compare(p(&Dict{}, Integer(0)), p(&Dict{}, Integer(0))); err == nil {
		t.Fatal("comparing Dict fields must fail")
	}
	h1, err1 := UserHash(p(Integer(1), String("a")))
	h2, err2 := UserHash(p(Float(1), String("a")))
	if err1 != nil || err2 != nil || h1 != h2 {
		t.Fatalf("equal values must hash alike: %d %v, %d %v", h1, err1, h2, err2)
	}
	if _, err := UserHash(p(&List{}, Nil)); err == nil || !errors.Is(err, TypeError) {
		t.Fatalf("an unhashable field must raise TypeError, got %v", err)
	}
	if s, err := UserToString(p(String("a"), &List{Elements: []Object{Integer(1)}})); err != nil || s != `Point(x="a", y=[1])` {
		t.Fatalf("structural show = %q, %v", s, err)
	}
	if got := point.Traits(); len(got) != 4 || got[0] != EqTrait || got[3] != ShowTrait {
		t.Fatalf("traits = %v", got)
	}
}

func TestOrdOverCompareEq(t *testing.T) {
	calls := 0
	money := sealed(t, "Money", []string{"amount"},
		ImplSpec{Trait: OrdTrait, Methods: []ImplMethod{
			method("compare", 2, func(args []Object) (Object, error) {
				calls++
				return Minus(args[0].(UserValue).FieldValues()[0], args[1])
			}),
		}},
		ImplSpec{Trait: EqTrait, Methods: []ImplMethod{
			method("eq", 2, func(args []Object) (Object, error) {
				c, err := OrdTrait.call(OrdCompare, args)
				if err != nil {
					return nil, err
				}
				return Bool(c.(Integer) == 0), nil
			}),
		}},
	)
	m := &fakeValue{typ: money, fields: []Object{Integer(5)}}
	if eq, err := Equals(m, Integer(5)); err != nil || !eq {
		t.Fatalf("eq from compare = %v, %v", eq, err)
	}
	// compare raises TypeError for nil, which reads as unequal.
	if eq, err := Equals(m, Nil); err != nil || eq {
		t.Fatalf("eq against nil = %v, %v", eq, err)
	}
	// 10 > m asks m for the mirrored question.
	if gt, err := Greater(Integer(10), m); err != nil || !gt {
		t.Fatalf("reflected ordering = %v, %v", gt, err)
	}
	if calls == 0 {
		t.Fatal("compare never ran")
	}
}

func TestStrictReturnTypes(t *testing.T) {
	liar := sealed(t, "Liar", nil,
		ImplSpec{Trait: EqTrait, Methods: []ImplMethod{method("eq", 2, func([]Object) (Object, error) { return Integer(1), nil })}},
		ImplSpec{Trait: ShowTrait, Methods: []ImplMethod{method("show", 1, func([]Object) (Object, error) { return Nil, nil })}},
	)
	v := &fakeValue{typ: liar}
	_, err := Equals(v, Integer(1))
	if err == nil || err.Error() != "Liar.eq must return Bool, got Integer" || !errors.Is(err, TypeError) {
		t.Fatalf("a wrong eq result must not read as unequal, got %v", err)
	}
	if _, err := UserToString(v); err == nil || err.Error() != "Liar.show must return String, got Nil" {
		t.Fatalf("show result error = %v", err)
	}
	if s := UserString(v); !strings.HasPrefix(s, "<Liar@") {
		t.Fatalf("infallible String must fall back to the default repr, got %q", s)
	}
}

func TestArithDispatch(t *testing.T) {
	vec := sealed(t, "Vec", []string{"x"},
		ImplSpec{Trait: AddTrait, Methods: []ImplMethod{
			method("add", 2, func(args []Object) (Object, error) {
				x, err := Add(args[0].(UserValue).FieldValues()[0], args[1].(UserValue).FieldValues()[0])
				return &fakeValue{typ: args[0].(UserValue).UserType(), fields: []Object{x}}, err
			}),
		}},
		ImplSpec{Trait: NegTrait, Methods: []ImplMethod{
			method("neg", 1, func(args []Object) (Object, error) {
				return &fakeValue{typ: args[0].(UserValue).UserType(), fields: []Object{-args[0].(UserValue).FieldValues()[0].(Integer)}}, nil
			}),
		}},
		ImplSpec{Trait: MulTrait, Methods: []ImplMethod{
			method("mul", 2, func(args []Object) (Object, error) { return String("mul"), nil }),
			method("rmul", 2, func(args []Object) (Object, error) { return String("rmul"), nil }),
		}},
	)
	v := func(x int64) *fakeValue { return &fakeValue{typ: vec, fields: []Object{Integer(x)}} }

	if sum, err := Add(v(5), v(3)); err != nil || sum.(UserValue).FieldValues()[0] != Integer(8) {
		t.Fatalf("add = %v, %v", sum, err)
	}
	// Sub is a trait of its own: add and neg do not imply it.
	if _, err := Minus(v(5), v(3)); err == nil || err.Error() != "cannot subtract Vec" {
		t.Fatalf("missing sub = %v", err)
	}
	if r, err := Multiply(Integer(2), v(1)); err != nil || r != String("rmul") {
		t.Fatalf("reflected mul = %v, %v", r, err)
	}
	if _, err := Divide(v(1), Integer(2)); err == nil || err.Error() != "cannot divide Vec" {
		t.Fatalf("missing div = %v", err)
	}
	if _, err := Add(Integer(1), v(1)); err == nil || err.Error() != "cannot add Integer and Vec" {
		t.Fatalf("missing radd must keep the left error, got %v", err)
	}
	if n, err := Negate(v(4)); err != nil || n.(UserValue).FieldValues()[0] != Integer(-4) {
		t.Fatalf("negate = %v, %v", n, err)
	}
}

func TestTraitObjectOnBuiltinValues(t *testing.T) {
	call := func(trait *Trait, name string, args ...Object) (Object, error) {
		result, handled, err := trait.CallMethod(name, CallArgs{Positional: args})
		if !handled {
			t.Fatalf("%s has no method %s", trait.Name, name)
		}
		return result, err
	}
	cases := []struct {
		trait *Trait
		name  string
		args  []Object
		want  string
	}{
		{EqTrait, "eq", []Object{Integer(1), Float(1)}, "true"},
		{EqTrait, "ne", []Object{String("a"), String("b")}, "true"},
		{OrdTrait, "compare", []Object{Integer(3), Integer(2)}, "1"},
		{OrdTrait, "le", []Object{String("a"), String("a")}, "true"},
		{OrdTrait, "max", []Object{Integer(1), Integer(1)}, "1"},
		{OrdTrait, "min", []Object{Float(2), Integer(1)}, "1"},
		{ShowTrait, "show", []Object{&List{Elements: []Object{String("x")}}}, `["x"]`},
		{TruthTrait, "truth", []Object{String("")}, "false"},
		{SubTrait, "rsub", []Object{Integer(1), Integer(10)}, "9"},
		{NegTrait, "neg", []Object{Float(2)}, "-2"},
		{IterTrait, "iter", []Object{String("ab")}, `["a", "b"]`},
		{IndexTrait, "get", []Object{&List{Elements: []Object{Integer(7)}}, Integer(0)}, "7"},
	}
	for _, tc := range cases {
		got, err := call(tc.trait, tc.name, tc.args...)
		if err != nil || inspect(got) != tc.want {
			t.Errorf("%s.%s(%v) = %v, %v; want %s", tc.trait.Name, tc.name, tc.args, got, err, tc.want)
		}
	}
	if h, err := call(HashableTrait, "hash", Integer(1)); err != nil || h.TypeName() != "Integer" {
		t.Errorf("Hashable.hash(1) = %v, %v", h, err)
	}
	if _, err := call(HashableTrait, "hash", &List{}); err == nil || err.Error() != "List does not implement Hashable" {
		t.Errorf("Hashable.hash([]) error = %v", err)
	}
	if _, err := call(OrdTrait, "max", Integer(1)); err == nil || err.Error() != "Ord.max() takes 2 positional arguments, got 1" {
		t.Errorf("arity error = %v", err)
	}
	user := NewTrait("Shape", nil, []TraitMethod{{Name: "area", Arity: 1, Required: true}})
	if _, err := call(user, "area", Integer(1)); err == nil || err.Error() != "Integer does not implement Shape" {
		t.Errorf("user trait on a built-in = %v", err)
	}
}

func TestCheckImpls(t *testing.T) {
	shape := NewTrait("Shape", []*Trait{ShowTrait}, []TraitMethod{
		{Name: "area", Arity: 1, Required: true},
		{Name: "describe", Arity: 1},
	})
	spec := func(trait *Trait, methods ...ImplMethod) ImplSpec { return ImplSpec{Trait: trait, Methods: methods} }
	named := func(name string, arity int) ImplMethod { return ImplMethod{Name: name, Arity: arity} }
	cases := []struct {
		impls []ImplSpec
		want  string
	}{
		{[]ImplSpec{spec(ShowTrait), spec(shape, named("area", 1))}, ""},
		{[]ImplSpec{spec(shape, named("area", 1))}, "impl Shape for T requires impl Show"},
		{[]ImplSpec{spec(ShowTrait), spec(shape)}, "impl Shape for T is missing method 'area'"},
		{[]ImplSpec{spec(ShowTrait), spec(shape, named("area", 2))}, "impl Shape for T: method 'area' must declare 1 parameters including self, got 2"},
		{[]ImplSpec{spec(ShowTrait), spec(ShowTrait)}, "duplicate impl Show for T"},
		{[]ImplSpec{spec(AddTrait)}, "impl Add for T is missing method 'add'"},
		{[]ImplSpec{spec(OrdTrait, named("compare", 2), named("max", 2))}, "impl Ord for T: method 'max' derives from the required methods and cannot be overridden"},
		{[]ImplSpec{spec(EqTrait), spec(OrdTrait), spec(HashableTrait)}, ""},
		{[]ImplSpec{spec(OrdTrait)}, "impl Ord for T requires impl Eq"},
		{[]ImplSpec{spec(EqTrait, named("eq", 2)), spec(OrdTrait)}, "structural Ord requires structural Eq on T"},
		{[]ImplSpec{spec(EqTrait, named("eq", 2)), spec(OrdTrait, named("compare", 2)), spec(HashableTrait, named("hash", 1))}, ""},
	}
	for i, tc := range cases {
		issue := CheckImpls("T", tc.impls)
		got := ""
		if issue != nil {
			got = issue.Message
		}
		if got != tc.want {
			t.Errorf("case %d: got %q, want %q", i, got, tc.want)
		}
	}
}

func TestReviewRegressions(t *testing.T) {
	// A user type named like a built-in must not pass the return type check.
	impostorType := sealed(t, "String", nil)
	liar := sealed(t, "Liar", nil, ImplSpec{Trait: ShowTrait, Methods: []ImplMethod{
		method("show", 1, func([]Object) (Object, error) { return &fakeValue{typ: impostorType}, nil }),
	}})
	if _, err := UserToString(&fakeValue{typ: liar}); err == nil || !errors.Is(err, TypeError) {
		t.Fatalf("impostor String result = %v", err)
	}

	// A failure on the right side of a reflected ordering is reported.
	boom := sealed(t, "Boom", nil, ImplSpec{Trait: EqTrait}, ImplSpec{Trait: OrdTrait, Methods: []ImplMethod{
		method("compare", 2, func([]Object) (Object, error) { return nil, NewZeroDivisionError("division by zero") }),
	}})
	if _, err := Less(Integer(1), &fakeValue{typ: boom}); err == nil || !errors.Is(err, ZeroDivisionError) {
		t.Fatalf("right-hand failure = %v", err)
	}

	// A zero-parameter trait method (possible without the checker) fails cleanly.
	zero := NewTrait("Z", nil, []TraitMethod{{Name: "m", Arity: 0, Required: true}})
	if _, _, err := zero.CallMethod("m", CallArgs{}); err == nil {
		t.Fatal("calling a method without a receiver must fail")
	}
}

func TestBuiltinValueMissingTrait(t *testing.T) {
	fn := &Function{Name: "f"}
	call := func(tr *Trait, method string, args ...Object) error {
		i, _ := tr.MethodIndex(method)
		_, err := tr.Invoke(i, CallArgs{Positional: args})
		return err
	}
	cases := []struct {
		err  error
		want string
	}{
		{call(OrdTrait, "compare", fn, fn), "Function does not implement Ord"},
		{call(OrdTrait, "lt", &Dict{}, &Dict{}), "Dict does not implement Ord"},
		{call(NegTrait, "neg", String("a")), "String does not implement Neg"},
		// The receiver implements the trait; only the operand is wrong.
		{call(OrdTrait, "compare", &List{}, &Dict{}), "cannot compare List and Dict"},
		// radd on an Integer computes fn + 1: the failing type is not the receiver.
		{call(AddTrait, "radd", Integer(1), fn), "cannot add Function"},
	}
	for _, tc := range cases {
		if tc.err == nil || tc.err.Error() != tc.want {
			t.Errorf("error = %v, want %q", tc.err, tc.want)
		}
		if !errors.Is(tc.err, TypeError) {
			t.Errorf("%v should be a TypeError", tc.err)
		}
	}
}
