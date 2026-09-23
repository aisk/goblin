package object

import (
	"errors"
	"fmt"
)

// Trait is the runtime value of a trait: a named bundle of methods a user type
// implements in an impl block. Built-in traits (Eq, Ord, ...) back operators
// and conversions; user traits come from `trait` declarations. Trait objects
// are ordinary values: `Ord.max(a, b)` reads the method off the trait and
// calls it with the receiver as the first argument, so the call works on
// user values and built-in values alike.
//
// Identity is the trait: impls are recorded by *Trait, never by name, so two
// modules declaring a trait of the same name never collide.
type Trait struct {
	NoReflectedOps
	NoAssignment
	Name    string
	Deps    []*Trait
	Methods []TraitMethod

	index map[string]int
	bound []*Function
	// route answers a method for a receiver that is not a user value, by
	// method index. Only built-in traits have one; a nil entry runs the
	// method's default instead.
	route []func(args []Object) (Object, error)
	// structural holds, by method index, the implementation an impl that
	// supplies none of the required methods receives. Only Eq, Ord, Hashable
	// and Show have one.
	structural []*Function
}

// TraitMethod describes one method of a trait. Arity counts self. A required
// method has no default; Num's methods are all optional, and their defaults
// raise the operator's TypeError.
type TraitMethod struct {
	Name     string
	Arity    int
	Required bool
	// Derived marks a default an impl may not override: it is defined by the
	// required methods, so an override could only disagree with them.
	Derived bool
	Default *Function
	// Returns is the type name an implementation's result must have, or ""
	// when any value is accepted.
	Returns string
}

var _ Object = (*Trait)(nil)

// NewTrait builds a trait from its declaration.
func NewTrait(name string, deps []*Trait, methods []TraitMethod) *Trait {
	t := &Trait{Name: name}
	t.init(deps, methods)
	return t
}

func (t *Trait) init(deps []*Trait, methods []TraitMethod) {
	t.Deps = deps
	t.Methods = methods
	t.index = make(map[string]int, len(methods))
	t.bound = make([]*Function, len(methods))
	for i, m := range methods {
		t.index[m.Name] = i
		i := i
		t.bound[i] = &Function{Name: t.Name + "." + m.Name, Fn: func(args CallArgs) (Object, error) {
			return t.Invoke(i, args)
		}}
	}
}

// MethodIndex locates a method by name.
func (t *Trait) MethodIndex(name string) (int, bool) {
	i, ok := t.index[name]
	return i, ok
}

// Invoke performs `Trait.method(args)`: the argument count is checked against
// the declared arity, then the first argument's implementation runs.
func (t *Trait) Invoke(i int, args CallArgs) (Object, error) {
	m := &t.Methods[i]
	if len(args.Keyword) > 0 {
		return nil, NewTypeError("%s.%s() got an unexpected keyword argument '%s'", t.Name, m.Name, args.Keyword[0].Name)
	}
	if len(args.Positional) != m.Arity || m.Arity == 0 {
		return nil, NewTypeError("%s.%s() takes %d positional arguments, got %d", t.Name, m.Name, m.Arity, len(args.Positional))
	}
	return t.call(i, args.Positional)
}

// call dispatches method i on args[0] without checking the argument count.
// A user value runs its impl (or the default); a built-in value goes through
// the trait's route.
func (t *Trait) call(i int, args []Object) (Object, error) {
	recv := args[0]
	if uv, ok := recv.(UserValue); ok {
		impl := uv.UserType().Impl(t)
		if impl == nil {
			return nil, NewTypeError(ErrFmtNotImplemented, recv.TypeName(), t.Name)
		}
		return impl.call(recv, i, args)
	}
	if t.route != nil {
		if r := t.route[i]; r != nil {
			result, err := r(args)
			if err != nil && isUnsupported(err, recv) {
				return nil, NewTypeError(ErrFmtNotImplemented, recv.TypeName(), t.Name)
			}
			return result, err
		}
		if d := t.Methods[i].Default; d != nil {
			return d.Call(CallArgs{Positional: args})
		}
	}
	return nil, NewTypeError(ErrFmtNotImplemented, recv.TypeName(), t.Name)
}

func (t *Trait) TypeName() string { return "Trait" }

func (t *Trait) String() string { return fmt.Sprintf("<trait %s>", t.Name) }

func (t *Trait) ToString() (string, error) { return t.String(), nil }
func (t *Trait) ToBool() (bool, error)     { return true, nil }

// Equals is identity: a trait equals only itself.
func (t *Trait) Equals(other Object) (bool, error) {
	v, ok := other.(*Trait)
	return ok && t == v, nil
}

// Hash follows identity, so trait objects can be dict keys.
func (t *Trait) Hash() (uint64, error) { return identityHash(t), nil }

func (t *Trait) Compare(Object) (int, error) {
	return 0, NewUnsupportedError("Trait", ErrFmtCannotCompare, "Trait")
}
func (t *Trait) Add(Object) (Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtCannotAdd, "Trait")
}
func (t *Trait) Minus(Object) (Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtCannotSubtract, "Trait")
}
func (t *Trait) Multiply(Object) (Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtCannotMultiply, "Trait")
}
func (t *Trait) Divide(Object) (Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtCannotDivide, "Trait")
}
func (t *Trait) Modulo(Object) (Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtCannotModulo, "Trait")
}
func (t *Trait) Iter() ([]Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtNotIterable, "Trait")
}
func (t *Trait) Index(Object) (Object, error) {
	return nil, NewUnsupportedError("Trait", ErrFmtNotIndexable, "Trait")
}

func (t *Trait) GetAttr(name string) (Object, error) {
	if i, ok := t.index[name]; ok {
		return t.bound[i], nil
	}
	if name == "attributes" {
		return AttributesFunction(t), nil
	}
	return nil, NewAttributeError("trait %s has no method '%s'", t.Name, name)
}

// CallMethod satisfies MethodCaller, so `Ord.max(a, b)` skips the bound
// function GetAttr hands out.
func (t *Trait) CallMethod(name string, args CallArgs) (Object, bool, error) {
	i, ok := t.index[name]
	if !ok {
		return nil, false, nil
	}
	v, err := t.Invoke(i, args)
	return v, true, err
}

func (t *Trait) Attributes() []string {
	names := make([]string, 0, len(t.Methods)+1)
	for _, m := range t.Methods {
		names = append(names, m.Name)
	}
	return append(names, "attributes")
}

// AsTrait checks that an impl block or a dependency list names a trait. site
// says where the reference appears ("impl Shape", "trait Titled") and ref is
// the reference as written; both go into the message.
func AsTrait(v Object, site, ref string) (*Trait, error) {
	if t, ok := v.(*Trait); ok {
		return t, nil
	}
	return nil, NewTypeError(ErrFmtNotATrait, site, ref)
}

// ModuleTrait resolves a `module.Name` trait reference.
func ModuleTrait(module Object, name, site, ref string) (*Trait, error) {
	v, err := module.GetAttr(name)
	if err != nil {
		return nil, NewNameError(ErrFmtTraitNotDefined, site, ref)
	}
	return AsTrait(v, site, ref)
}

// errBadReturn marks the TypeError raised when a trait method returns a value
// of the wrong type. It is a real TypeError for `catch`, but equality and the
// reflected operators must not read it as "this operand does not know the
// other one": it is a bug in the implementation, not an answer.
var errBadReturn = errors.New("bad trait method return type")

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
