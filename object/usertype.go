package object

import (
	"fmt"
)

// UserValue is an instance of a Goblin `type`. The interpreter's instances and
// the structs the transpiler generates both implement it by embedding
// UserBase, whose Object methods call the user* helpers below, so operator
// and conversion dispatch through protocols is written once.
type UserValue interface {
	Object
	UserType() *UserType
	// FieldValues returns the field values in declaration order. Callers must
	// not modify the slice.
	FieldValues() []Object
}

// UserType is the runtime description of a Goblin `type` that protocol
// dispatch needs: its name, its fields, and its impls.
type UserType struct {
	Name   string
	Fields []string
	// Constructor is the type's constructor, what type_of answers for its
	// values. Both backends set it when they define the type.
	Constructor *Function

	pending []ImplSpec
	// impls holds the type's impl of each protocol, indexed by Protocol.id,
	// so an operator finds its impl with one index.
	impls [numProtocols]*ProtocolImpl
}

// ProtocolImpl is one type's implementation of one protocol.
type ProtocolImpl struct {
	Protocol *Protocol
	// methods holds, by method index, the implementation the impl supplied
	// or the structural one it received; nil for an optional method the
	// impl leaves out.
	methods []*Function
	// structural is set when the impl supplied none of the required methods
	// and received the protocol's structural ones.
	structural bool
}

// ImplSpec describes an impl block before it is validated: the protocol and
// the methods the block defines, in source order.
type ImplSpec struct {
	Protocol *Protocol
	Methods  []ImplMethod
}

// ImplMethod is one method of an impl block. Arity counts self; Fn takes self
// as its first positional argument. The semantic checker validates impls
// without functions, so Fn may be nil there.
type ImplMethod struct {
	Name  string
	Arity int
	Fn    *Function
}

// NewUserType creates the description of a type without impls.
func NewUserType(name string, fields []string) *UserType {
	return &UserType{Name: name, Fields: fields}
}

// Implement records an impl block. Nothing is checked until Seal, so the
// blocks of a type may be recorded in any order.
func (t *UserType) Implement(p *Protocol, methods []ImplMethod) {
	t.pending = append(t.pending, ImplSpec{Protocol: p, Methods: methods})
}

// Seal validates the recorded impls and builds the dispatch table, filling in
// structural implementations.
func (t *UserType) Seal() error {
	specs := t.pending
	t.pending = nil
	if issue := CheckImpls(t.Name, specs); issue != nil {
		return NewTypeError("%s", issue.Message)
	}
	for _, spec := range specs {
		p := spec.Protocol
		impl := &ProtocolImpl{Protocol: p, methods: make([]*Function, len(p.Methods))}
		for _, m := range spec.Methods {
			i, _ := p.MethodIndex(m.Name)
			impl.methods[i] = m.Fn
		}
		if p.structural != nil && !suppliesRequired(spec) {
			impl.structural = true
			for i, fn := range p.structural {
				if fn != nil {
					impl.methods[i] = fn
				}
			}
		}
		t.impls[p.id] = impl
	}
	return nil
}

// Implements reports whether the type has an impl of p.
func (t *UserType) Implements(p *Protocol) bool {
	return t.impls[p.id] != nil
}

// call runs method i of the impl on args (args[0] is recv) and validates the
// result type. The method must be present: required ones always are, and
// callers check supplies for an optional one.
func (impl *ProtocolImpl) call(recv Object, i int, args []Object) (Object, error) {
	m := &impl.Protocol.Methods[i]
	result, err := impl.methods[i].Call(CallArgs{Positional: args})
	if err != nil {
		return nil, err
	}
	if !returnsType(m.Returns, result) {
		return nil, newReturnTypeError(recv, m.Name, m.Returns, result)
	}
	return result, nil
}

// returnsType reports whether result has the type a protocol method must
// return. It checks the Go type: a user type may be named Bool or String too.
func returnsType(want string, result Object) bool {
	var ok bool
	switch want {
	case "":
		return true
	case "Bool":
		_, ok = result.(Bool)
	case "Int":
		_, ok = result.(Integer)
	case "String":
		_, ok = result.(String)
	}
	return ok
}

// supplies reports whether the impl defines method i itself (or received it
// structurally), which an optional method may not.
func (impl *ProtocolImpl) supplies(i int) bool { return impl.methods[i] != nil }

func suppliesRequired(spec ImplSpec) bool {
	for _, m := range spec.Methods {
		if i, ok := spec.Protocol.MethodIndex(m.Name); ok && spec.Protocol.Methods[i].Required {
			return true
		}
	}
	return false
}

// ImplIssue is a problem CheckImpls found. Impl indexes the offending impl and
// Method the offending method within it, -1 when the impl as a whole is at
// fault.
type ImplIssue struct {
	Impl    int
	Method  int
	Message string
}

// CheckImpls validates a type's impls: every method belongs to its protocol
// with the declared arity, every required method is present (or the
// protocol's structural implementation applies), dependencies are
// implemented, and structural Ord and Hashable only sit on a structural Eq. The semantic checker and
// UserType.Seal share it, so both report identical messages.
func CheckImpls(typeName string, impls []ImplSpec) *ImplIssue {
	seen := make(map[*Protocol]int, len(impls))
	structural := make(map[*Protocol]bool, len(impls))
	for i, spec := range impls {
		tr := spec.Protocol
		if _, dup := seen[tr]; dup {
			return &ImplIssue{i, -1, fmt.Sprintf("duplicate impl %s for %s", tr.Name, typeName)}
		}
		seen[tr] = i
		names := make(map[string]bool, len(spec.Methods))
		for j, m := range spec.Methods {
			if names[m.Name] {
				return &ImplIssue{i, j, fmt.Sprintf("duplicate method '%s' in impl %s for %s", m.Name, tr.Name, typeName)}
			}
			names[m.Name] = true
			k, ok := tr.MethodIndex(m.Name)
			if !ok {
				return &ImplIssue{i, j, fmt.Sprintf("impl %s for %s: %s has no method '%s'", tr.Name, typeName, tr.Name, m.Name)}
			}
			if want := tr.Methods[k].Arity; m.Arity != want {
				return &ImplIssue{i, j, fmt.Sprintf("impl %s for %s: method '%s' must declare %d parameters including self, got %d", tr.Name, typeName, m.Name, want, m.Arity)}
			}
		}
		if tr.structural != nil && !suppliesRequired(spec) {
			structural[tr] = true
			continue
		}
		for _, m := range tr.Methods {
			if m.Required && !names[m.Name] {
				return &ImplIssue{i, -1, fmt.Sprintf("impl %s for %s is missing method '%s'", tr.Name, typeName, m.Name)}
			}
		}
	}

	for i, spec := range impls {
		for _, dep := range spec.Protocol.Deps {
			if _, ok := seen[dep]; ok {
				continue
			}
			return &ImplIssue{i, -1, fmt.Sprintf("impl %s for %s requires impl %s", spec.Protocol.Name, typeName, dep.Name)}
		}
	}

	for _, tr := range []*Protocol{OrdProtocol, HashableProtocol} {
		i, ok := seen[tr]
		if !ok || !structural[tr] || structural[EqProtocol] {
			continue
		}
		return &ImplIssue{i, -1, fmt.Sprintf("structural %s requires structural Eq on %s", tr.Name, typeName)}
	}
	return nil
}

// defaultRepr is how a value without Show prints.
func defaultRepr(v UserValue) string {
	return fmt.Sprintf("<%s@%p>", v.TypeName(), v)
}

// userString is the infallible String of a user value: its Show, or the
// default representation when there is none or it fails. A show that calls
// os.exit ends the process here, with the code and without unwinding: there
// is no error result to carry the request, and dropping it would keep
// running a program that asked to stop.
func userString(v UserValue) string {
	if impl := v.UserType().impls[showID]; impl != nil {
		r, err := impl.call(v, ShowShow, []Object{v})
		if err == nil {
			return string(r.(String))
		}
		ExitNow(err)
	}
	return defaultRepr(v)
}

// userToString performs Str(v), print and interpolation through Show.
func userToString(v UserValue) (string, error) {
	impl := v.UserType().impls[showID]
	if impl == nil {
		return defaultRepr(v), nil
	}
	r, err := impl.call(v, ShowShow, []Object{v})
	if err != nil {
		return "", err
	}
	return string(r.(String)), nil
}

// userToBool is the truth value through Truth; without it a value is true.
func userToBool(v UserValue) (bool, error) {
	impl := v.UserType().impls[truthID]
	if impl == nil {
		return true, nil
	}
	r, err := impl.call(v, TruthTruth, []Object{v})
	if err != nil {
		return false, err
	}
	return bool(r.(Bool)), nil
}

// userEquals answers one side of == through Eq; without it a value is equal
// only to itself.
func userEquals(v UserValue, other Object) (bool, error) {
	impl := v.UserType().impls[eqID]
	if impl == nil {
		return other == Object(v), nil
	}
	r, err := impl.call(v, EqEq, []Object{v, other})
	if err != nil {
		return false, err
	}
	return bool(r.(Bool)), nil
}

// userCompare orders v against other through Ord.compare.
func userCompare(v UserValue, other Object) (int, error) {
	impl := v.UserType().impls[ordID]
	if impl == nil {
		return 0, missingImpl(v, ErrFmtCannotCompare, "Ord")
	}
	r, err := impl.call(v, OrdCompare, []Object{v, other})
	if err != nil {
		return 0, err
	}
	return int(r.(Integer)), nil
}

var arithErrFmts = [...]string{
	ArithAdd: ErrFmtCannotAdd,
	ArithSub: ErrFmtCannotSubtract,
	ArithMul: ErrFmtCannotMultiply,
	ArithDiv: ErrFmtCannotDivide,
	ArithMod: ErrFmtCannotModulo,
}

var arithProtocols = [...]string{
	ArithAdd: "Add",
	ArithSub: "Sub",
	ArithMul: "Mul",
	ArithDiv: "Div",
	ArithMod: "Mod",
}

// missingImplHint follows the message of an operation whose protocol a user
// type does not implement, naming the impl to add.
const missingImplHint = " (add 'impl %s' to type %s)"

// missingImpl is the TypeError for an operation v's type has no impl of
// protocol for: format, which takes the type name, plus the hint.
func missingImpl(v UserValue, format, protocol string) *Error {
	tn := v.TypeName()
	return NewTypeError(format+missingImplHint, tn, protocol, tn)
}

// userArith performs a binary operator with v on the left through its
// arithmetic protocol. op is ArithAdd, ArithSub, ArithMul, ArithDiv or ArithMod.
func userArith(v UserValue, op int, other Object) (Object, error) {
	impl := v.UserType().impls[addID+op]
	if impl == nil {
		return nil, missingImpl(v, arithErrFmts[op], arithProtocols[op])
	}
	return impl.call(v, ArithForward, []Object{v, other})
}

// userReflected performs a binary operator with v on the right, through the
// protocol's reflected method (radd ... rmod). handled is false when the impl
// does not define it, so the left operand's error stands.
func userReflected(v UserValue, op int, left Object) (result Object, handled bool, err error) {
	impl := v.UserType().impls[addID+op]
	if impl == nil || !impl.supplies(ArithReflected) {
		return nil, false, nil
	}
	result, err = impl.call(v, ArithReflected, []Object{v, left})
	return result, true, err
}

// userIterator starts a pass over the values `for x in v` visits: Iter.iter
// returns an iterable, often a List, and the pass delegates to it. A lazy
// result, such as a file, stays lazy, and since the returned *Iteration is a
// Closer, closing this pass closes the delegated one.
func userIterator(v UserValue) (Iterator, error) {
	impl := v.UserType().impls[iterID]
	if impl == nil {
		return nil, missingImpl(v, ErrFmtNotIterable, "Iter")
	}
	r, err := impl.call(v, IterIter, []Object{v})
	if err != nil {
		return nil, err
	}
	it, err := Iterate(r)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// userIter lists the values `for x in v` visits, through Iter.iter.
func userIter(v UserValue) ([]Object, error) {
	impl := v.UserType().impls[iterID]
	if impl == nil {
		return nil, missingImpl(v, ErrFmtNotIterable, "Iter")
	}
	r, err := impl.call(v, IterIter, []Object{v})
	if err != nil {
		return nil, err
	}
	return Collect(r)
}

// userIndex performs v[index] through Index.get.
func userIndex(v UserValue, index Object) (Object, error) {
	impl := v.UserType().impls[indexID]
	if impl == nil {
		return nil, missingImpl(v, ErrFmtNotIndexable, "Index")
	}
	return impl.call(v, IndexGet, []Object{v, index})
}

// userSetIndex performs v[index] = value through Index.set. handled is false
// without one, leaving SetIndex to raise the shared error.
func userSetIndex(v UserValue, index, value Object) (bool, error) {
	impl := v.UserType().impls[indexID]
	if impl == nil || !impl.supplies(IndexSet) {
		return false, nil
	}
	_, err := impl.call(v, IndexSet, []Object{v, index, value})
	return true, err
}

// userHash hashes v as a dict key through Hashable.
func userHash(v UserValue) (uint64, error) {
	impl := v.UserType().impls[hashableID]
	if impl == nil {
		return 0, missingImpl(v, "cannot hash %s", "Hashable")
	}
	r, err := impl.call(v, HashableHash, []Object{v})
	if err != nil {
		return 0, err
	}
	return uint64(r.(Integer)), nil
}

// structuralShow renders `Name(field=value, ...)`. Values render the way a
// collection renders its elements: strings quoted, everything else through
// Show.
func structuralShow(v UserValue) (Object, error) {
	s, err := (&renderer{strict: true}).user(v)
	return String(s), err
}
