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

const (
	NumAdd = iota
	NumSub
	NumMul
	NumDiv
	NumMod
	NumNeg
	NumRAdd
	NumRSub
	NumRMul
	NumRDiv
	NumRMod
)

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
	NumTrait      = &Trait{Name: "Num"}
	IterTrait     = &Trait{Name: "Iter"}
	IndexTrait    = &Trait{Name: "Index"}
)

// BuiltinTraits lists the built-in traits by their global names.
var BuiltinTraits = map[string]*Trait{
	"Eq":       EqTrait,
	"Ord":      OrdTrait,
	"Hashable": HashableTrait,
	"Show":     ShowTrait,
	"Truth":    TruthTrait,
	"Num":      NumTrait,
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
	initNum()
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
		{Name: "ne", Arity: 2, Returns: "Bool", Default: goFn("ne", func(args []Object) (Object, error) {
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
		return TraitMethod{Name: name, Arity: 2, Returns: "Bool", Default: goFn(name, func(args []Object) (Object, error) {
			c, err := OrdTrait.call(OrdCompare, args)
			if err != nil {
				return nil, err
			}
			return Bool(op.test(int(c.(Integer)))), nil
		})}
	}
	// max and min follow Haskell: on a tie max answers other and min self.
	pick := func(name string, max bool) TraitMethod {
		return TraitMethod{Name: name, Arity: 2, Default: goFn(name, func(args []Object) (Object, error) {
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

func initNum() {
	unsupported := func(name, format string) *Function {
		return goFn(name, func(args []Object) (Object, error) {
			return nil, NewTypeError(format, args[0].TypeName())
		})
	}
	binary := func(name, format string) TraitMethod {
		return TraitMethod{Name: name, Arity: 2, Default: unsupported(name, format)}
	}
	// sub derives from add and neg when the impl supplies both.
	sub := TraitMethod{Name: "sub", Arity: 2, Default: goFn("sub", func(args []Object) (Object, error) {
		if uv, ok := args[0].(UserValue); ok {
			if impl := uv.UserType().num; impl != nil && impl.supplies(NumAdd) && impl.supplies(NumNeg) {
				neg, err := NumTrait.call(NumNeg, args[1:])
				if err != nil {
					return nil, err
				}
				return impl.call(uv, NumAdd, []Object{uv, neg})
			}
		}
		return nil, NewTypeError(ErrFmtCannotSubtract, args[0].TypeName())
	})}
	NumTrait.init(nil, []TraitMethod{
		binary("add", ErrFmtCannotAdd),
		sub,
		binary("mul", ErrFmtCannotMultiply),
		binary("div", ErrFmtCannotDivide),
		binary("mod", ErrFmtCannotModulo),
		{Name: "neg", Arity: 1, Default: unsupported("neg", ErrFmtCannotNegate)},
		binary("radd", ErrFmtCannotAdd),
		binary("rsub", ErrFmtCannotSubtract),
		binary("rmul", ErrFmtCannotMultiply),
		binary("rdiv", ErrFmtCannotDivide),
		binary("rmod", ErrFmtCannotModulo),
	})
	ops := []func(a, b Object) (Object, error){Add, Minus, Multiply, Divide, Modulo}
	NumTrait.route = make([]func([]Object) (Object, error), len(NumTrait.Methods))
	for i, op := range ops {
		op := op
		NumTrait.route[NumAdd+i] = func(args []Object) (Object, error) { return op(args[0], args[1]) }
		NumTrait.route[NumRAdd+i] = func(args []Object) (Object, error) { return op(args[1], args[0]) }
	}
	NumTrait.route[NumNeg] = func(args []Object) (Object, error) { return Negate(args[0]) }
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

// eqFromOrd is the Eq a type implementing Ord without Eq receives: compare
// answering 0. A compare that does not know other reads as unequal.
var eqFromOrd = goFn("eq", func(args []Object) (Object, error) {
	c, err := OrdTrait.call(OrdCompare, args)
	if err != nil {
		if notHandled(err) {
			return False, nil
		}
		return nil, err
	}
	return Bool(c.(Integer) == 0), nil
})

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
