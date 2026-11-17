package object

import (
	"errors"
	"fmt"
	"hash/maphash"
	"strings"
)

// Protocol describes one language protocol: the closed set of names an
// `impl` block may implement, each connecting a user type to a piece of
// syntax or a built-in conversion (Eq to ==, Add to +, Iter to for, ...).
// A protocol is not a value; it exists only after `impl` and in Go.
type Protocol struct {
	Name string
	// id indexes UserType.impls.
	id      int
	Deps    []*Protocol
	Methods []ProtocolMethod
	// structural holds, by method index, the implementation an impl that
	// supplies none of the required methods receives. Only Eq, Ord, Hashable
	// and Show have one.
	structural []*Function
}

// ProtocolMethod describes one method an impl of a protocol may define.
// Arity counts self.
type ProtocolMethod struct {
	Name     string
	Arity    int
	Required bool
	// Returns is the type name an implementation's result must have, or ""
	// when any value is accepted.
	Returns string
}

// MethodIndex locates a method by name.
func (p *Protocol) MethodIndex(name string) (int, bool) {
	for i := range p.Methods {
		if p.Methods[i].Name == name {
			return i, true
		}
	}
	return 0, false
}

// Method indexes of the protocols.
const (
	EqEq         = 0
	OrdCompare   = 0
	HashableHash = 0
	ShowShow     = 0
	TruthTruth   = 0
	NegNeg       = 0
	IterIter     = 0
)

const (
	IndexGet = iota
	IndexSet
)

// The binary arithmetic operators, in the order of ArithProtocols.
const (
	ArithAdd = iota
	ArithSub
	ArithMul
	ArithDiv
	ArithMod
)

// Method indexes of the binary arithmetic protocols: the operator with the
// value on the left, then the optional reflected one with the value on the
// right.
const (
	ArithForward = iota
	ArithReflected
)

// Protocol ids; the arithmetic ones are consecutive, in ArithAdd order, so an
// operator finds its impl by offset.
const (
	eqID = iota
	ordID
	hashableID
	showID
	truthID
	addID
	subID
	mulID
	divID
	modID
	negID
	iterID
	indexID
	numProtocols
)

func binary(name string) ProtocolMethod { return ProtocolMethod{Name: name, Arity: 2, Required: true} }

func arith(id int, name string) *Protocol {
	return &Protocol{Name: strings.ToUpper(name[:1]) + name[1:], id: id, Methods: []ProtocolMethod{
		binary(name),
		{Name: "r" + name, Arity: 2},
	}}
}

// The language protocols.
var (
	EqProtocol = &Protocol{Name: "Eq", id: eqID,
		Methods:    []ProtocolMethod{{Name: "eq", Arity: 2, Required: true, Returns: "Bool"}},
		structural: []*Function{goFn("eq", structuralEq)},
	}
	OrdProtocol = &Protocol{Name: "Ord", id: ordID, Deps: []*Protocol{EqProtocol},
		Methods:    []ProtocolMethod{{Name: "compare", Arity: 2, Required: true, Returns: "Int"}},
		structural: []*Function{goFn("compare", structuralCompare)},
	}
	HashableProtocol = &Protocol{Name: "Hashable", id: hashableID, Deps: []*Protocol{EqProtocol},
		Methods:    []ProtocolMethod{{Name: "hash", Arity: 1, Required: true, Returns: "Int"}},
		structural: []*Function{goFn("hash", structuralHash)},
	}
	ShowProtocol = &Protocol{Name: "Show", id: showID,
		Methods: []ProtocolMethod{{Name: "show", Arity: 1, Required: true, Returns: "String"}},
		structural: []*Function{goFn("show", func(args []Object) (Object, error) {
			return structuralShow(args[0].(UserValue))
		})},
	}
	TruthProtocol = &Protocol{Name: "Truth", id: truthID,
		Methods: []ProtocolMethod{{Name: "truth", Arity: 1, Required: true, Returns: "Bool"}},
	}
	AddProtocol = arith(addID, "add")
	SubProtocol = arith(subID, "sub")
	MulProtocol = arith(mulID, "mul")
	DivProtocol = arith(divID, "div")
	ModProtocol = arith(modID, "mod")
	NegProtocol = &Protocol{Name: "Neg", id: negID,
		Methods: []ProtocolMethod{{Name: "neg", Arity: 1, Required: true}},
	}
	IterProtocol = &Protocol{Name: "Iter", id: iterID,
		Methods: []ProtocolMethod{{Name: "iter", Arity: 1, Required: true}},
	}
	IndexProtocol = &Protocol{Name: "Index", id: indexID,
		Methods: []ProtocolMethod{
			{Name: "get", Arity: 2, Required: true},
			{Name: "set", Arity: 3},
		},
	}
)

// ArithProtocols are the binary arithmetic protocols, indexed by ArithAdd ...
// ArithMod.
var ArithProtocols = [...]*Protocol{AddProtocol, SubProtocol, MulProtocol, DivProtocol, ModProtocol}

// protocols lists every protocol in the order messages name them.
var protocols = []*Protocol{
	EqProtocol, OrdProtocol, HashableProtocol, ShowProtocol, TruthProtocol,
	AddProtocol, SubProtocol, MulProtocol, DivProtocol, ModProtocol,
	NegProtocol, IterProtocol, IndexProtocol,
}

// LookupProtocol resolves the name written after `impl` in the definition of
// typeName. The name is never looked up in a scope: a program may declare its
// own Show, and `impl Show` still means the protocol. The semantic checker and
// both backends call it, so the REPL, which skips the checker, reports the
// same message.
func LookupProtocol(typeName, name string) (*Protocol, error) {
	names := make([]string, len(protocols))
	for i, p := range protocols {
		if p.Name == name {
			return p, nil
		}
		names[i] = p.Name
	}
	return nil, NewTypeError("impl %s for %s: %s is not a protocol (protocols: %s); other methods go directly in the type body",
		name, typeName, name, strings.Join(names, ", "))
}

// goFn wraps a Go implementation as a protocol method. Dispatch passes the
// arguments the method's arity promises, so implementations index them
// directly.
func goFn(name string, fn func(args []Object) (Object, error)) *Function {
	return &Function{Name: name, Fn: func(args CallArgs) (Object, error) {
		return fn(args.Positional)
	}}
}

// errBadReturn marks the TypeError raised when a protocol method returns a
// value of the wrong type. It is a real TypeError for `catch`, but equality
// and the reflected operators must not read it as "this operand does not know
// the other one": it is a bug in the implementation, not an answer.
var errBadReturn = errors.New("bad protocol method return type")

func newReturnTypeError(recv Object, method, want string, got Object) error {
	gotName := got.TypeName()
	if _, ok := got.(UserValue); ok {
		gotName = "user type " + gotName
	}
	msg := fmt.Sprintf(ErrFmtMustReturn, recv.TypeName(), method, want, gotName)
	return &Error{Value: msg, Wrapped: typedCause{cause: errBadReturn, base: TypeError}}
}

// notHandled reports whether err means "this operand has no answer for that
// one", the only failure that lets a symmetric operator ask the other side.
func notHandled(err error) bool {
	return errors.Is(err, TypeError) && !errors.Is(err, errBadReturn)
}

// sameUserType reports whether other is a value of v's type.
func sameUserType(v UserValue, other Object) (UserValue, bool) {
	ou, ok := other.(UserValue)
	return ou, ok && ou.UserType() == v.UserType()
}

func structuralEq(args []Object) (Object, error) {
	if err := CheckStack(); err != nil {
		return nil, err
	}
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
	if err := CheckStack(); err != nil {
		return nil, err
	}
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
	if err := CheckStack(); err != nil {
		return nil, err
	}
	v := args[0].(UserValue)
	const prime = 1099511628211
	h := maphash.String(hashSeed, v.TypeName())
	for _, field := range v.FieldValues() {
		fh, err := Hash(field)
		if err != nil {
			return nil, err
		}
		h = (h ^ fh) * prime
	}
	return Integer(int64(h)), nil
}
