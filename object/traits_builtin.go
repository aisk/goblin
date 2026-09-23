package object

import (
	"hash/maphash"
)

// Method indexes of the built-in traits, in declaration order.
const (
	EqEq = iota
	EqNe
)

const (
	OrdCompare = iota
	OrdLt
	OrdLe
	OrdGt
	OrdGe
	OrdMax
	OrdMin
)

const HashableHash = 0

const ShowShow = 0

const TruthTruth = 0

// The binary arithmetic operators, in the order of ArithTraits.
const (
	ArithAdd = iota
	ArithSub
	ArithMul
	ArithDiv
	ArithMod
)

// Method indexes of the binary arithmetic traits: the operator with the value
// on the left, then the reflected one with the value on the right.
const (
	ArithForward = iota
	ArithReflected
)

const NegNeg = 0

const IterIter = 0

const (
	IndexGet = iota
	IndexSet
)

// The built-in traits. The values are allocated here and filled in by init,
// because their defaults dispatch through the traits themselves, which a
// package-level initializer could not refer to.
var (
	EqTrait       = &Trait{Name: "Eq"}
	OrdTrait      = &Trait{Name: "Ord"}
	HashableTrait = &Trait{Name: "Hashable"}
	ShowTrait     = &Trait{Name: "Show"}
	TruthTrait    = &Trait{Name: "Truth"}
	AddTrait      = &Trait{Name: "Add"}
	SubTrait      = &Trait{Name: "Sub"}
	MulTrait      = &Trait{Name: "Mul"}
	DivTrait      = &Trait{Name: "Div"}
	ModTrait      = &Trait{Name: "Mod"}
	NegTrait      = &Trait{Name: "Neg"}
	IterTrait     = &Trait{Name: "Iter"}
	IndexTrait    = &Trait{Name: "Index"}
)

// ArithTraits are the binary arithmetic traits, indexed by ArithAdd ...
// ArithMod.
var ArithTraits = [...]*Trait{AddTrait, SubTrait, MulTrait, DivTrait, ModTrait}

// BuiltinTraits lists the built-in traits by their global names.
var BuiltinTraits = map[string]*Trait{
	"Eq":       EqTrait,
	"Ord":      OrdTrait,
	"Hashable": HashableTrait,
	"Show":     ShowTrait,
	"Truth":    TruthTrait,
	"Add":      AddTrait,
	"Sub":      SubTrait,
	"Mul":      MulTrait,
	"Div":      DivTrait,
	"Mod":      ModTrait,
	"Neg":      NegTrait,
	"Iter":     IterTrait,
	"Index":    IndexTrait,
}

// goFn wraps a Go implementation as a trait method function. Trait dispatch
// checks the argument count before a method runs, so the implementations index
// their arguments directly.
func goFn(name string, fn func(args []Object) (Object, error)) *Function {
	return &Function{Name: name, Fn: func(args CallArgs) (Object, error) {
		return fn(args.Positional)
	}}
}

func init() {
	initEq()
	initOrd()
	initHashable()
	initShow()
	TruthTrait.init(nil, []TraitMethod{{Name: "truth", Arity: 1, Required: true, Returns: "Bool"}})
	TruthTrait.route = []func([]Object) (Object, error){func(args []Object) (Object, error) {
		b, err := args[0].ToBool()
		return Bool(b), err
	}}
	initArith()
	IterTrait.init(nil, []TraitMethod{{Name: "iter", Arity: 1, Required: true}})
	IterTrait.route = []func([]Object) (Object, error){func(args []Object) (Object, error) {
		items, err := args[0].Iter()
		if err != nil {
			return nil, err
		}
		return &List{Elements: items}, nil
	}}
	initIndex()
}

func initEq() {
	EqTrait.init(nil, []TraitMethod{
		{Name: "eq", Arity: 2, Required: true, Returns: "Bool"},
		{Name: "ne", Arity: 2, Returns: "Bool", Derived: true, Default: goFn("ne", func(args []Object) (Object, error) {
			eq, err := EqTrait.call(EqEq, args)
			if err != nil {
				return nil, err
			}
			return Bool(!bool(eq.(Bool))), nil
		})},
	})
	EqTrait.route = []func([]Object) (Object, error){
		func(args []Object) (Object, error) {
			eq, err := Equals(args[0], args[1])
			return Bool(eq), err
		},
		nil,
	}
	EqTrait.structural = []*Function{goFn("eq", structuralEq), nil}
}

func initOrd() {
	test := func(name string, op orderOp) TraitMethod {
		return TraitMethod{Name: name, Arity: 2, Returns: "Bool", Derived: true, Default: goFn(name, func(args []Object) (Object, error) {
			c, err := OrdTrait.call(OrdCompare, args)
			if err != nil {
				return nil, err
			}
			return Bool(op.test(int(c.(Integer)))), nil
		})}
	}
	// max and min follow Haskell: on a tie max answers other and min self.
	pick := func(name string, max bool) TraitMethod {
		return TraitMethod{Name: name, Arity: 2, Derived: true, Default: goFn(name, func(args []Object) (Object, error) {
			le, err := OrdTrait.call(OrdLe, args)
			if err != nil {
				return nil, err
			}
			if bool(le.(Bool)) == max {
				return args[1], nil
			}
			return args[0], nil
		})}
	}
	OrdTrait.init([]*Trait{EqTrait}, []TraitMethod{
		{Name: "compare", Arity: 2, Required: true, Returns: "Integer"},
		test("lt", opLt), test("le", opLe), test("gt", opGt), test("ge", opGe),
		pick("max", true), pick("min", false),
	})
	OrdTrait.route = make([]func([]Object) (Object, error), len(OrdTrait.Methods))
	OrdTrait.route[OrdCompare] = func(args []Object) (Object, error) {
		c, err := Compare(args[0], args[1])
		return Integer(c), err
	}
	OrdTrait.structural = make([]*Function, len(OrdTrait.Methods))
	OrdTrait.structural[OrdCompare] = goFn("compare", structuralCompare)
}

func initHashable() {
	HashableTrait.init([]*Trait{EqTrait}, []TraitMethod{
		{Name: "hash", Arity: 1, Required: true, Returns: "Integer"},
	})
	HashableTrait.route = []func([]Object) (Object, error){func(args []Object) (Object, error) {
		h, ok := args[0].(Hashable)
		if !ok {
			return nil, NewTypeError(ErrFmtNotImplemented, args[0].TypeName(), "Hashable")
		}
		sum, err := h.Hash()
		return Integer(int64(sum)), err
	}}
	HashableTrait.structural = []*Function{goFn("hash", structuralHash)}
}

func initShow() {
	ShowTrait.init(nil, []TraitMethod{{Name: "show", Arity: 1, Required: true, Returns: "String"}})
	ShowTrait.route = []func([]Object) (Object, error){func(args []Object) (Object, error) {
		s, err := args[0].ToString()
		return String(s), err
	}}
	ShowTrait.structural = []*Function{goFn("show", func(args []Object) (Object, error) {
		return structuralShow(args[0].(UserValue))
	})}
}

func initArith() {
	ops := [...]struct {
		name, errFmt string
		op           func(a, b Object) (Object, error)
	}{
		ArithAdd: {"add", ErrFmtCannotAdd, Add},
		ArithSub: {"sub", ErrFmtCannotSubtract, Minus},
		ArithMul: {"mul", ErrFmtCannotMultiply, Multiply},
		ArithDiv: {"div", ErrFmtCannotDivide, Divide},
		ArithMod: {"mod", ErrFmtCannotModulo, Modulo},
	}
	for i, o := range ops {
		o := o
		// The reflected method is optional; without it a value on the right
		// leaves the left operand's error standing, and calling it directly
		// raises the operator's TypeError.
		reflected := "r" + o.name
		ArithTraits[i].init(nil, []TraitMethod{
			{Name: o.name, Arity: 2, Required: true},
			{Name: reflected, Arity: 2, Default: goFn(reflected, func(args []Object) (Object, error) {
				return nil, NewTypeError(o.errFmt, args[0].TypeName())
			})},
		})
		ArithTraits[i].route = []func([]Object) (Object, error){
			func(args []Object) (Object, error) { return o.op(args[0], args[1]) },
			func(args []Object) (Object, error) { return o.op(args[1], args[0]) },
		}
	}
	NegTrait.init(nil, []TraitMethod{{Name: "neg", Arity: 1, Required: true}})
	NegTrait.route = []func([]Object) (Object, error){func(args []Object) (Object, error) { return Negate(args[0]) }}
}

func initIndex() {
	IndexTrait.init(nil, []TraitMethod{
		{Name: "get", Arity: 2, Required: true},
		// set has no usable default: an impl without it is read-only.
		{Name: "set", Arity: 3, Default: goFn("set", func(args []Object) (Object, error) {
			return nil, NewTypeError("%s does not support index assignment", args[0].TypeName())
		})},
	})
	IndexTrait.route = []func([]Object) (Object, error){
		func(args []Object) (Object, error) { return args[0].Index(args[1]) },
		func(args []Object) (Object, error) {
			if err := SetIndex(args[0], args[1], args[2]); err != nil {
				return nil, err
			}
			return Nil, nil
		},
	}
}

// sameUserType reports whether other is a value of v's type.
func sameUserType(v UserValue, other Object) (UserValue, bool) {
	ou, ok := other.(UserValue)
	return ou, ok && ou.UserType() == v.UserType()
}

func structuralEq(args []Object) (Object, error) {
	v := args[0].(UserValue)
	other, ok := sameUserType(v, args[1])
	if !ok {
		return False, nil
	}
	theirs := other.FieldValues()
	for i, mine := range v.FieldValues() {
		eq, err := Equals(mine, theirs[i])
		if err != nil || !eq {
			return False, err
		}
	}
	return True, nil
}

func structuralCompare(args []Object) (Object, error) {
	v := args[0].(UserValue)
	other, ok := sameUserType(v, args[1])
	if !ok {
		return nil, NewTypeError("cannot compare %s and %s", v.TypeName(), args[1].TypeName())
	}
	theirs := other.FieldValues()
	for i, mine := range v.FieldValues() {
		c, err := Compare(mine, theirs[i])
		if err != nil {
			return nil, err
		}
		if c != 0 {
			return Integer(c), nil
		}
	}
	return Integer(0), nil
}

// structuralHash combines the type name with each field's hash, FNV style.
func structuralHash(args []Object) (Object, error) {
	v := args[0].(UserValue)
	const prime = 1099511628211
	h := maphash.String(hashSeed, v.TypeName())
	for _, field := range v.FieldValues() {
		fh, err := hashKey(field)
		if err != nil {
			return nil, err
		}
		h = (h ^ fh) * prime
	}
	return Integer(int64(h)), nil
}
