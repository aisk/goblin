package object

import "fmt"

type Object interface {
	// ToString performs Goblin's string conversion protocol. It may invoke a
	// user-defined __str method and propagate its error. Types additionally
	// implement fmt.Stringer for the infallible representation used by
	// diagnostics (see inspect).
	ToString() (string, error)
	ToBool() (bool, error)
	// Equals reports whether the receiver equals other. Unrelated types are
	// simply unequal, so a built-in never fails here; the error exists for a
	// user-defined __cmp, whose failure must not be mistaken for "not equal".
	// A TypeError is the exception: it reads as "I do not know this type",
	// which the package-level Equals turns back into "unequal". That function
	// is what the == operator uses, and it consults both operands, so an
	// implementation only needs to recognize the types it considers equal to
	// itself. Types without a natural equality fall back to identity (see
	// instance and generated user types).
	Equals(other Object) (bool, error)
	// Compare orders the receiver against other for <, <=, > and >=. It
	// fails for operands without a defined ordering; equality must not rely
	// on it.
	Compare(other Object) (int, error)
	Add(other Object) (Object, error)
	Minus(other Object) (Object, error)
	Multiply(other Object) (Object, error)
	Divide(other Object) (Object, error)
	Modulo(other Object) (Object, error)
	// RAdd, RMinus, RMultiply, RDivide and RModulo are the reflected halves of the
	// arithmetic operators, reached when the receiver stands on the right of
	// an operand that does not recognize it: `1 + money` asks money. The
	// argument is the LEFT operand, so RMinus(left) computes `left - receiver`.
	// The bool reports whether the receiver handled this operand at all; when
	// it is false the left operand's error stands. Types without a reflected
	// form of an operator embed NoReflectedOps or answer false.
	RAdd(left Object) (Object, bool, error)
	RMinus(left Object) (Object, bool, error)
	RMultiply(left Object) (Object, bool, error)
	RDivide(left Object) (Object, bool, error)
	RModulo(left Object) (Object, bool, error)
	Not() (Object, error)
	Iter() ([]Object, error)
	Index(index Object) (Object, error)
	GetAttr(name string) (Object, error)
	Attributes() []string
	// SetIndex and SetAttr perform `value[index] = x` and `value.name = x`.
	// The bool reports whether the receiver supports that form of assignment
	// at all; when it is false the caller raises the type error, so a value
	// that never accepts assignment embeds NoAssignment and says nothing.
	SetIndex(index Object, value Object) (bool, error)
	SetAttr(name string, value Object) (bool, error)
	// TypeName is the Goblin-level name of the value's type, as diagnostics
	// spell it. Every type names itself: a Go type name would differ between
	// the interpreter and transpiled programs (*interpreter.instance vs
	// *main.Point), making the two backends disagree on an error's text.
	TypeName() string
}

// inspect returns an infallible representation for diagnostics, formatting,
// and code paths that must always be able to produce text. Every object type
// provides it by implementing fmt.Stringer; ToString is the failing,
// __str-dispatching counterpart.
func inspect(obj Object) string {
	if s, ok := obj.(fmt.Stringer); ok {
		return s.String()
	}
	return obj.TypeName()
}

// literal returns an object's representation inside a collection literal.
// Strings need quoting; other objects already provide an appropriate String
// representation, including nested collections.
func literal(obj Object) string {
	if s, ok := obj.(String); ok {
		return s.Literal()
	}
	return inspect(obj)
}

// literalString is literal's failing twin, used by the collections' ToString.
// Rendering a collection runs the __str of every value inside it, and those
// may fail; a collection must not swallow what its elements report.
func literalString(obj Object) (string, error) {
	if s, ok := obj.(String); ok {
		return s.Literal(), nil
	}
	return obj.ToString()
}

// AttributesFunction exposes an object's attribute names as the bound
// attributes() method. A fresh List is returned on every call so callers
// cannot mutate shared runtime metadata.
func AttributesFunction(obj Object) *Function {
	return &Function{Name: "attributes", Fn: func(args CallArgs) (Object, error) {
		if err := RequireNoArgs("attributes", args); err != nil {
			return nil, err
		}
		names := obj.Attributes()
		elements := make([]Object, len(names))
		for i, name := range names {
			elements[i] = String(name)
		}
		return &List{Elements: elements}, nil
	}}
}

// MethodCaller is implemented by types that can run one of their own methods
// without handing out a bound *Function first. handled reports whether the
// name is one of the receiver's methods; when it is false the caller falls
// back to GetAttr, so fields holding a function, "constructor", "attributes"
// and unknown-attribute errors all keep their original behavior.
type MethodCaller interface {
	CallMethod(name string, args CallArgs) (Object, bool, error)
}

// CallMethod performs `obj.name(args)`. The built-in collection types are
// dispatched by concrete type rather than through the MethodCaller interface:
// an interface call is opaque to escape analysis, and args flowing into one is
// enough to push every call site's argument literal onto the heap. The two
// paths that do go through something opaque hand over a copy for the same
// reason.
func CallMethod(obj Object, name string, args CallArgs) (Object, error) {
	switch v := obj.(type) {
	case *List:
		if result, handled, err := v.CallMethod(name, args); handled {
			return result, err
		}
	case String:
		if result, handled, err := v.CallMethod(name, args); handled {
			return result, err
		}
	case *Dict:
		if result, handled, err := v.CallMethod(name, args); handled {
			return result, err
		}
	case Bytes:
		if result, handled, err := v.CallMethod(name, args); handled {
			return result, err
		}
	case MethodCaller:
		if result, handled, err := v.CallMethod(name, args.copy()); handled {
			return result, err
		}
	}
	attr, err := obj.GetAttr(name)
	if err != nil {
		return nil, err
	}
	return Call(attr, args.copy())
}

func Call(obj Object, args CallArgs) (Object, error) {
	switch v := obj.(type) {
	case *Function:
		return v.Call(args)
	}
	return nil, NewTypeError("%s is not callable", inspect(obj))
}

// NoReflectedOps answers "not handled" for every reflected operator. Types
// with no reflected form embed it instead of spelling out five methods that
// say nothing; a type that supports one operator from the right embeds it and
// overrides just that method. Go cannot embed into a named non-struct type, so
// the scalar built-ins declare the methods themselves.
type NoReflectedOps struct{}

func (NoReflectedOps) RAdd(Object) (Object, bool, error)      { return nil, false, nil }
func (NoReflectedOps) RMinus(Object) (Object, bool, error)    { return nil, false, nil }
func (NoReflectedOps) RMultiply(Object) (Object, bool, error) { return nil, false, nil }
func (NoReflectedOps) RDivide(Object) (Object, bool, error)   { return nil, false, nil }
func (NoReflectedOps) RModulo(Object) (Object, bool, error)   { return nil, false, nil }

// NoAssignment declines both forms of assignment. Types that accept one embed
// it and override just that method, the way *List overrides SetIndex.
type NoAssignment struct{}

func (NoAssignment) SetIndex(Object, Object) (bool, error) { return false, nil }
func (NoAssignment) SetAttr(string, Object) (bool, error)  { return false, nil }

// SetIndex performs an index assignment. A value that does not accept one is
// reported by its type, not by its contents, so the message reads the same for
// every value of that type.
func SetIndex(obj Object, index Object, value Object) error {
	handled, err := obj.SetIndex(index, value)
	if err != nil {
		return err
	}
	if !handled {
		return NewTypeError("%s does not support index assignment", obj.TypeName())
	}
	return nil
}

// IndexInt performs `obj[i]` for an index the caller already holds as a native
// integer. Lists are the overwhelmingly common receiver, so they are handled
// here directly without boxing the index and dispatching through the
// interface; every other receiver, including user types with __getitem, sees
// exactly the boxed call it would have seen anyway.
func IndexInt(obj Object, i int64) (Object, error) {
	if l, ok := obj.(*List); ok && i >= 0 && i < int64(len(l.Elements)) {
		return l.Elements[i], nil
	}
	return obj.Index(Integer(i))
}

// SetIndexInt performs `obj[i] = value` for a native integer index, with the
// same list fast path as IndexInt.
func SetIndexInt(obj Object, i int64, value Object) error {
	if l, ok := obj.(*List); ok && i >= 0 && i < int64(len(l.Elements)) {
		l.Elements[i] = value
		return nil
	}
	return SetIndex(obj, Integer(i), value)
}

// SetAttr performs a member assignment.
func SetAttr(obj Object, name string, value Object) error {
	handled, err := obj.SetAttr(name, value)
	if err != nil {
		return err
	}
	if !handled {
		return NewTypeError("%s does not support attribute assignment", obj.TypeName())
	}
	return nil
}
