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
func (v *fakeValue) String() string        { return userString(v) }
func (v *fakeValue) ToString() (string, error) {
	return userToString(v)
}
func (v *fakeValue) ToBool() (bool, error)             { return userToBool(v) }
func (v *fakeValue) Hash() (uint64, error)             { return userHash(v) }
func (v *fakeValue) Equals(o Object) (bool, error)     { return userEquals(v, o) }
func (v *fakeValue) Compare(o Object) (int, error)     { return userCompare(v, o) }
func (v *fakeValue) Add(o Object) (Object, error)      { return userArith(v, ArithAdd, o) }
func (v *fakeValue) Minus(o Object) (Object, error)    { return userArith(v, ArithSub, o) }
func (v *fakeValue) Multiply(o Object) (Object, error) { return userArith(v, ArithMul, o) }
func (v *fakeValue) Divide(o Object) (Object, error)   { return userArith(v, ArithDiv, o) }
func (v *fakeValue) Modulo(o Object) (Object, error)   { return userArith(v, ArithMod, o) }
func (v *fakeValue) RAdd(o Object) (Object, bool, error) {
	return userReflected(v, ArithAdd, o)
}
func (v *fakeValue) RMinus(o Object) (Object, bool, error) {
	return userReflected(v, ArithSub, o)
}
func (v *fakeValue) RMultiply(o Object) (Object, bool, error) {
	return userReflected(v, ArithMul, o)
}
func (v *fakeValue) RDivide(o Object) (Object, bool, error) {
	return userReflected(v, ArithDiv, o)
}
func (v *fakeValue) RModulo(o Object) (Object, bool, error) {
	return userReflected(v, ArithMod, o)
}
func (v *fakeValue) Iter() ([]Object, error)            { return userIter(v) }
func (v *fakeValue) Index(i Object) (Object, error)     { return userIndex(v, i) }
func (v *fakeValue) GetAttr(string) (Object, error)     { return nil, NewAttributeError("none") }
func (v *fakeValue) Attributes() []string               { return nil }
func (v *fakeValue) SetIndex(i, x Object) (bool, error) { return userSetIndex(v, i, x) }

// sealed builds a type from impls, failing the test when they do not validate.
func sealed(t *testing.T, name string, fields []string, impls ...ImplSpec) *UserType {
	t.Helper()
	typ := NewUserType(name, fields)
	for _, impl := range impls {
		typ.Implement(impl.Protocol, impl.Methods)
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
		ImplSpec{Protocol: EqProtocol}, ImplSpec{Protocol: OrdProtocol}, ImplSpec{Protocol: HashableProtocol}, ImplSpec{Protocol: ShowProtocol})
	p := func(x, y Object) *fakeValue { return &fakeValue{typ: point, fields: []Object{x, y}} }

	if eq, err := Equals(p(Integer(1), String("a")), p(Float(1), String("a"))); err != nil || !eq {
		t.Fatalf("structural eq = %v, %v", eq, err)
	}
	other := sealed(t, "Other", []string{"x", "y"}, ImplSpec{Protocol: EqProtocol})
	if eq, _ := Equals(p(Integer(1), Integer(2)), &fakeValue{typ: other, fields: []Object{Integer(1), Integer(2)}}); eq {
		t.Fatal("values of different types must be unequal")
	}
	if lt, err := Less(p(Integer(1), Integer(9)), p(Integer(2), Integer(0))); err != nil || !lt {
		t.Fatalf("lexicographic compare = %v, %v", lt, err)
	}
	if _, err := Compare(p(&Dict{}, Integer(0)), p(&Dict{}, Integer(0))); err == nil {
		t.Fatal("comparing Dict fields must fail")
	}
	h1, err1 := userHash(p(Integer(1), String("a")))
	h2, err2 := userHash(p(Float(1), String("a")))
	if err1 != nil || err2 != nil || h1 != h2 {
		t.Fatalf("equal values must hash alike: %d %v, %d %v", h1, err1, h2, err2)
	}
	if _, err := userHash(p(&List{}, Nil)); err == nil || !errors.Is(err, TypeError) {
		t.Fatalf("an unhashable field must raise TypeError, got %v", err)
	}
	if s, err := userToString(p(String("a"), &List{Elements: []Object{Integer(1)}})); err != nil || s != `Point(x="a", y=[1])` {
		t.Fatalf("structural show = %q, %v", s, err)
	}
	if !point.Implements(OrdProtocol) || other.Implements(OrdProtocol) {
		t.Fatal("Implements disagrees with the impls")
	}
}

func TestOrdOverCompareEq(t *testing.T) {
	calls := 0
	money := sealed(t, "Money", []string{"amount"},
		ImplSpec{Protocol: OrdProtocol, Methods: []ImplMethod{
			method("compare", 2, func(args []Object) (Object, error) {
				calls++
				return Minus(args[0].(UserValue).FieldValues()[0], args[1])
			}),
		}},
		ImplSpec{Protocol: EqProtocol, Methods: []ImplMethod{
			method("eq", 2, func(args []Object) (Object, error) {
				c, err := Compare(args[0], args[1])
				if err != nil {
					return nil, err
				}
				return Bool(c == 0), nil
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
		ImplSpec{Protocol: EqProtocol, Methods: []ImplMethod{method("eq", 2, func([]Object) (Object, error) { return Integer(1), nil })}},
		ImplSpec{Protocol: ShowProtocol, Methods: []ImplMethod{method("show", 1, func([]Object) (Object, error) { return Nil, nil })}},
	)
	v := &fakeValue{typ: liar}
	_, err := Equals(v, Integer(1))
	if err == nil || err.Error() != "Liar.eq must return Bool, got Int" || !errors.Is(err, TypeError) {
		t.Fatalf("a wrong eq result must not read as unequal, got %v", err)
	}
	if _, err := userToString(v); err == nil || err.Error() != "Liar.show must return String, got Nil" {
		t.Fatalf("show result error = %v", err)
	}
	if s := userString(v); !strings.HasPrefix(s, "<Liar@") {
		t.Fatalf("infallible String must fall back to the default repr, got %q", s)
	}
}

func TestArithDispatch(t *testing.T) {
	vec := sealed(t, "Vec", []string{"x"},
		ImplSpec{Protocol: AddProtocol, Methods: []ImplMethod{
			method("add", 2, func(args []Object) (Object, error) {
				x, err := Add(args[0].(UserValue).FieldValues()[0], args[1].(UserValue).FieldValues()[0])
				return &fakeValue{typ: args[0].(UserValue).UserType(), fields: []Object{x}}, err
			}),
		}},
		ImplSpec{Protocol: NegProtocol, Methods: []ImplMethod{
			method("neg", 1, func(args []Object) (Object, error) {
				return &fakeValue{typ: args[0].(UserValue).UserType(), fields: []Object{-args[0].(UserValue).FieldValues()[0].(Integer)}}, nil
			}),
		}},
		ImplSpec{Protocol: MulProtocol, Methods: []ImplMethod{
			method("mul", 2, func(args []Object) (Object, error) { return String("mul"), nil }),
			method("rmul", 2, func(args []Object) (Object, error) { return String("rmul"), nil }),
		}},
	)
	v := func(x int64) *fakeValue { return &fakeValue{typ: vec, fields: []Object{Integer(x)}} }

	if sum, err := Add(v(5), v(3)); err != nil || sum.(UserValue).FieldValues()[0] != Integer(8) {
		t.Fatalf("add = %v, %v", sum, err)
	}
	// Sub is a protocol of its own: add and neg do not imply it.
	if _, err := Minus(v(5), v(3)); err == nil || err.Error() != "cannot subtract Vec" {
		t.Fatalf("missing sub = %v", err)
	}
	if r, err := Multiply(Integer(2), v(1)); err != nil || r != String("rmul") {
		t.Fatalf("reflected mul = %v, %v", r, err)
	}
	if _, err := Divide(v(1), Integer(2)); err == nil || err.Error() != "cannot divide Vec" {
		t.Fatalf("missing div = %v", err)
	}
	if _, err := Add(Integer(1), v(1)); err == nil || err.Error() != "cannot add Int and Vec" {
		t.Fatalf("missing radd must keep the left error, got %v", err)
	}
	if n, err := Negate(v(4)); err != nil || n.(UserValue).FieldValues()[0] != Integer(-4) {
		t.Fatalf("negate = %v, %v", n, err)
	}
}

func TestCheckImpls(t *testing.T) {
	spec := func(p *Protocol, methods ...ImplMethod) ImplSpec { return ImplSpec{Protocol: p, Methods: methods} }
	named := func(name string, arity int) ImplMethod { return ImplMethod{Name: name, Arity: arity} }
	cases := []struct {
		impls []ImplSpec
		want  string
	}{
		{[]ImplSpec{spec(ShowProtocol), spec(IndexProtocol, named("get", 2))}, ""},
		{[]ImplSpec{spec(HashableProtocol, named("hash", 1))}, "impl Hashable for T requires impl Eq"},
		{[]ImplSpec{spec(TruthProtocol)}, "impl Truth for T is missing method 'truth'"},
		{[]ImplSpec{spec(IndexProtocol, named("get", 2), named("set", 2))}, "impl Index for T: method 'set' must declare 3 parameters including self, got 2"},
		{[]ImplSpec{spec(IterProtocol, named("iter", 1), named("iter", 1))}, "duplicate method 'iter' in impl Iter for T"},
		{[]ImplSpec{spec(MulProtocol, named("rmul", 2))}, "impl Mul for T is missing method 'mul'"},
		{[]ImplSpec{spec(ShowProtocol), spec(ShowProtocol)}, "duplicate impl Show for T"},
		{[]ImplSpec{spec(AddProtocol)}, "impl Add for T is missing method 'add'"},
		{[]ImplSpec{spec(OrdProtocol, named("compare", 2), named("lt", 2))}, "impl Ord for T: Ord has no method 'lt'"},
		{[]ImplSpec{spec(EqProtocol, named("eq", 2), named("ne", 2))}, "impl Eq for T: Eq has no method 'ne'"},
		{[]ImplSpec{spec(EqProtocol), spec(OrdProtocol), spec(HashableProtocol)}, ""},
		{[]ImplSpec{spec(OrdProtocol)}, "impl Ord for T requires impl Eq"},
		{[]ImplSpec{spec(EqProtocol, named("eq", 2)), spec(OrdProtocol)}, "structural Ord requires structural Eq on T"},
		{[]ImplSpec{spec(EqProtocol, named("eq", 2)), spec(OrdProtocol, named("compare", 2)), spec(HashableProtocol, named("hash", 1))}, ""},
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
	liar := sealed(t, "Liar", nil, ImplSpec{Protocol: ShowProtocol, Methods: []ImplMethod{
		method("show", 1, func([]Object) (Object, error) { return &fakeValue{typ: impostorType}, nil }),
	}})
	if _, err := userToString(&fakeValue{typ: liar}); err == nil || !errors.Is(err, TypeError) {
		t.Fatalf("impostor String result = %v", err)
	}

	// A failure on the right side of a reflected ordering is reported.
	boom := sealed(t, "Boom", nil, ImplSpec{Protocol: EqProtocol}, ImplSpec{Protocol: OrdProtocol, Methods: []ImplMethod{
		method("compare", 2, func([]Object) (Object, error) { return nil, NewZeroDivisionError("division by zero") }),
	}})
	if _, err := Less(Integer(1), &fakeValue{typ: boom}); err == nil || !errors.Is(err, ZeroDivisionError) {
		t.Fatalf("right-hand failure = %v", err)
	}
}

func TestLookupProtocol(t *testing.T) {
	for _, p := range []*Protocol{EqProtocol, OrdProtocol, HashableProtocol, ShowProtocol, TruthProtocol,
		AddProtocol, SubProtocol, MulProtocol, DivProtocol, ModProtocol, NegProtocol, IterProtocol, IndexProtocol} {
		got, err := LookupProtocol("T", p.Name)
		if err != nil || got != p {
			t.Errorf("LookupProtocol(%s) = %v, %v", p.Name, got, err)
		}
	}
	_, err := LookupProtocol("Rect", "Shape")
	want := "impl Shape for Rect: Shape is not a protocol (protocols: Eq, Ord, Hashable, Show, Truth, Add, Sub, Mul, Div, Mod, Neg, Iter, Index); other methods go directly in the type body"
	if err == nil || !errors.Is(err, TypeError) || err.Error() != want {
		t.Fatalf("LookupProtocol(Shape) = %v", err)
	}
}

func TestOptionalMethods(t *testing.T) {
	readOnly := sealed(t, "ReadOnly", nil, ImplSpec{Protocol: IndexProtocol, Methods: []ImplMethod{
		method("get", 2, func(args []Object) (Object, error) { return args[1], nil }),
	}})
	v := &fakeValue{typ: readOnly}
	if got, err := v.Index(Integer(3)); err != nil || got != Integer(3) {
		t.Fatalf("get = %v, %v", got, err)
	}
	if handled, err := userSetIndex(v, Integer(0), Nil); handled || err != nil {
		t.Fatalf("set without an impl method = %v, %v", handled, err)
	}
	if _, handled, err := userReflected(v, ArithAdd, Integer(1)); handled || err != nil {
		t.Fatalf("radd without an impl = %v, %v", handled, err)
	}
}
