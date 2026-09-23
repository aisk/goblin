package object

import (
	"fmt"
	"strings"
)

// UserValue is an instance of a Goblin `type`. The interpreter's instances and
// the structs the transpiler generates both implement it, and both implement
// their Object methods by calling the User* helpers below, so operator and
// conversion dispatch through traits is written once.
type UserValue interface {
	Object
	UserType() *UserType
	// FieldValues returns the field values in declaration order. Callers must
	// not modify the slice.
	FieldValues() []Object
}

// UserType is the runtime description of a Goblin `type` that trait dispatch
// needs: its name, its fields, and its impls.
type UserType struct {
	Name   string
	Fields []string

	pending []ImplSpec
	impls   []*TraitImpl
	byTrait map[*Trait]*TraitImpl
	// The built-in traits' impls, cached so an operator does not pay for a
	// map lookup.
	eq, ord, hashable, show, truth, neg, iter, index *TraitImpl
	arith                                            [len(ArithTraits)]*TraitImpl
}

// TraitImpl is one type's implementation of one trait.
type TraitImpl struct {
	Trait *Trait
	// methods holds, by method index, the implementation the impl supplied
	// or the structural one it received; nil means the trait's default.
	methods []*Function
}

// ImplSpec describes an impl block before it is validated: the trait and the
// methods the block defines, in source order.
type ImplSpec struct {
	Trait   *Trait
	Methods []ImplMethod
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
func (t *UserType) Implement(trait *Trait, methods []ImplMethod) {
	t.pending = append(t.pending, ImplSpec{Trait: trait, Methods: methods})
}

// Seal validates the recorded impls and builds the dispatch tables, filling in
// structural implementations.
func (t *UserType) Seal() error {
	specs := t.pending
	t.pending = nil
	if issue := CheckImpls(t.Name, specs); issue != nil {
		return NewTypeError("%s", issue.Message)
	}
	t.byTrait = make(map[*Trait]*TraitImpl, len(specs)+1)
	for _, spec := range specs {
		impl := &TraitImpl{Trait: spec.Trait, methods: make([]*Function, len(spec.Trait.Methods))}
		for _, m := range spec.Methods {
			i, _ := spec.Trait.MethodIndex(m.Name)
			impl.methods[i] = m.Fn
		}
		if spec.Trait.structural != nil && !suppliesRequired(spec) {
			for i, fn := range spec.Trait.structural {
				if fn != nil {
					impl.methods[i] = fn
				}
			}
		}
		t.add(impl)
	}
	return nil
}

func (t *UserType) add(impl *TraitImpl) {
	t.impls = append(t.impls, impl)
	t.byTrait[impl.Trait] = impl
	switch impl.Trait {
	case EqTrait:
		t.eq = impl
	case OrdTrait:
		t.ord = impl
	case HashableTrait:
		t.hashable = impl
	case ShowTrait:
		t.show = impl
	case TruthTrait:
		t.truth = impl
	case NegTrait:
		t.neg = impl
	case IterTrait:
		t.iter = impl
	case IndexTrait:
		t.index = impl
	}
	for i, tr := range ArithTraits {
		if impl.Trait == tr {
			t.arith[i] = impl
		}
	}
}

// Impl returns the type's implementation of trait, or nil.
func (t *UserType) Impl(trait *Trait) *TraitImpl {
	return t.byTrait[trait]
}

// Traits lists the implemented traits in declaration order.
func (t *UserType) Traits() []*Trait {
	traits := make([]*Trait, len(t.impls))
	for i, impl := range t.impls {
		traits[i] = impl.Trait
	}
	return traits
}

// call runs method i of the impl on args (args[0] is recv), falling back to
// the trait's default and validating the result type.
func (impl *TraitImpl) call(recv Object, i int, args []Object) (Object, error) {
	fn := impl.methods[i]
	m := &impl.Trait.Methods[i]
	if fn == nil {
		fn = m.Default
	}
	result, err := fn.Call(CallArgs{Positional: args})
	if err != nil {
		return nil, err
	}
	if !returnsType(m.Returns, result) {
		return nil, newReturnTypeError(recv, m.Name, m.Returns, result)
	}
	return result, nil
}

// returnsType reports whether result has the type a trait method must return.
// It checks the Go type: a user type may be named Bool or String too.
func returnsType(want string, result Object) bool {
	var ok bool
	switch want {
	case "":
		return true
	case "Bool":
		_, ok = result.(Bool)
	case "Integer":
		_, ok = result.(Integer)
	case "String":
		_, ok = result.(String)
	}
	return ok
}

// supplies reports whether the impl defines method i itself (or received it
// structurally), as opposed to relying on the trait's default.
func (impl *TraitImpl) supplies(i int) bool { return impl.methods[i] != nil }

func suppliesRequired(spec ImplSpec) bool {
	for _, m := range spec.Methods {
		if i, ok := spec.Trait.MethodIndex(m.Name); ok && spec.Trait.Methods[i].Required {
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

// CheckImpls validates a type's impls: every method belongs to its trait with
// the declared arity and overrides no derived method, every required method is present (or the trait's
// structural default applies), dependencies are implemented, and structural
// Ord and Hashable only sit on a structural Eq. The semantic checker and
// UserType.Seal share it, so both report identical messages.
func CheckImpls(typeName string, impls []ImplSpec) *ImplIssue {
	seen := make(map[*Trait]int, len(impls))
	structural := make(map[*Trait]bool, len(impls))
	for i, spec := range impls {
		tr := spec.Trait
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
			if tr.Methods[k].Derived {
				return &ImplIssue{i, j, fmt.Sprintf("impl %s for %s: method '%s' derives from the required methods and cannot be overridden", tr.Name, typeName, m.Name)}
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
		for _, dep := range spec.Trait.Deps {
			if _, ok := seen[dep]; ok {
				continue
			}
			return &ImplIssue{i, -1, fmt.Sprintf("impl %s for %s requires impl %s", spec.Trait.Name, typeName, dep.Name)}
		}
	}

	for _, tr := range []*Trait{OrdTrait, HashableTrait} {
		i, ok := seen[tr]
		if !ok || !structural[tr] || structural[EqTrait] {
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

// UserString is the infallible String of a user value: its Show, or the
// default representation when there is none or it fails.
func UserString(v UserValue) string {
	if impl := v.UserType().show; impl != nil {
		if r, err := impl.call(v, ShowShow, []Object{v}); err == nil {
			return string(r.(String))
		}
	}
	return defaultRepr(v)
}

// UserToString performs Str(v), print and interpolation through Show.
func UserToString(v UserValue) (string, error) {
	impl := v.UserType().show
	if impl == nil {
		return defaultRepr(v), nil
	}
	r, err := impl.call(v, ShowShow, []Object{v})
	if err != nil {
		return "", err
	}
	return string(r.(String)), nil
}

// UserToBool is the truth value through Truth; without it a value is true.
func UserToBool(v UserValue) (bool, error) {
	impl := v.UserType().truth
	if impl == nil {
		return true, nil
	}
	r, err := impl.call(v, TruthTruth, []Object{v})
	if err != nil {
		return false, err
	}
	return bool(r.(Bool)), nil
}

// UserEquals answers one side of == through Eq; without it a value is equal
// only to itself.
func UserEquals(v UserValue, other Object) (bool, error) {
	impl := v.UserType().eq
	if impl == nil {
		return other == Object(v), nil
	}
	r, err := impl.call(v, EqEq, []Object{v, other})
	if err != nil {
		return false, err
	}
	return bool(r.(Bool)), nil
}

// UserCompare orders v against other through Ord.compare.
func UserCompare(v UserValue, other Object) (int, error) {
	impl := v.UserType().ord
	if impl == nil {
		return 0, NewTypeError(ErrFmtCannotCompare, v.TypeName())
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

// UserArith performs a binary operator with v on the left through its
// arithmetic trait. op is ArithAdd, ArithSub, ArithMul, ArithDiv or ArithMod.
func UserArith(v UserValue, op int, other Object) (Object, error) {
	impl := v.UserType().arith[op]
	if impl == nil {
		return nil, NewTypeError(arithErrFmts[op], v.TypeName())
	}
	return impl.call(v, ArithForward, []Object{v, other})
}

// UserReflected performs a binary operator with v on the right, through the
// trait's reflected method (radd ... rmod). handled is false when the impl
// does not define it, so the left operand's error stands.
func UserReflected(v UserValue, op int, left Object) (result Object, handled bool, err error) {
	impl := v.UserType().arith[op]
	if impl == nil || !impl.supplies(ArithReflected) {
		return nil, false, nil
	}
	result, err = impl.call(v, ArithReflected, []Object{v, left})
	return result, true, err
}

// UserIter lists the values `for x in v` visits, through Iter.iter.
func UserIter(v UserValue) ([]Object, error) {
	impl := v.UserType().iter
	if impl == nil {
		return nil, NewTypeError(ErrFmtNotIterable, v.TypeName())
	}
	r, err := impl.call(v, IterIter, []Object{v})
	if err != nil {
		return nil, err
	}
	return r.Iter()
}

// UserIndex performs v[index] through Index.get.
func UserIndex(v UserValue, index Object) (Object, error) {
	impl := v.UserType().index
	if impl == nil {
		return nil, NewTypeError(ErrFmtNotIndexable, v.TypeName())
	}
	return impl.call(v, IndexGet, []Object{v, index})
}

// UserSetIndex performs v[index] = value through Index.set. handled is false
// without one, leaving SetIndex to raise the shared error.
func UserSetIndex(v UserValue, index, value Object) (bool, error) {
	impl := v.UserType().index
	if impl == nil || !impl.supplies(IndexSet) {
		return false, nil
	}
	_, err := impl.call(v, IndexSet, []Object{v, index, value})
	return true, err
}

// UserHash hashes v as a dict key through Hashable.
func UserHash(v UserValue) (uint64, error) {
	impl := v.UserType().hashable
	if impl == nil {
		return 0, NewTypeError("unhashable dict key: %s", inspect(v))
	}
	r, err := impl.call(v, HashableHash, []Object{v})
	if err != nil {
		return 0, err
	}
	return uint64(r.(Integer)), nil
}

// TraitsFunction exposes a user value's implemented traits as the bound
// traits() method. A fresh List is returned on every call.
func TraitsFunction(v UserValue) *Function {
	return &Function{Name: "traits", Fn: func(args CallArgs) (Object, error) {
		if err := RequireNoArgs("traits", args); err != nil {
			return nil, err
		}
		traits := v.UserType().Traits()
		elements := make([]Object, len(traits))
		for i, tr := range traits {
			elements[i] = tr
		}
		return &List{Elements: elements}, nil
	}}
}

// structuralShow renders `Name(field=value, ...)`. Values render the way a
// collection renders its elements: strings quoted, everything else through
// Show.
func structuralShow(v UserValue) (Object, error) {
	var b strings.Builder
	b.WriteString(v.TypeName())
	b.WriteByte('(')
	names := v.UserType().Fields
	for i, value := range v.FieldValues() {
		if i > 0 {
			b.WriteString(", ")
		}
		s, err := literalString(value)
		if err != nil {
			return nil, err
		}
		b.WriteString(names[i])
		b.WriteByte('=')
		b.WriteString(s)
	}
	b.WriteByte(')')
	return String(b.String()), nil
}
