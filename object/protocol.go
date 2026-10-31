package object

import (
	"errors"
	"fmt"
)

// Diagnostics raised by operator and trait dispatch. Both backends format them
// with the type's declared name, so an error reads identically whichever
// backend runs the program.
//
// The ErrFmtCannot* and ErrFmtNot* formats say that a type has no form of an
// operation at all and take the type name; raise them with
// NewUnsupportedError (or Unsupported). An operation the type has but not for
// the given operand reads "cannot add Integer and String" instead; raise it
// with NewOperandError.
const (
	ErrFmtCannotAdd      = "cannot add %s"
	ErrFmtCannotSubtract = "cannot subtract %s"
	ErrFmtCannotMultiply = "cannot multiply %s"
	ErrFmtCannotDivide   = "cannot divide %s"
	ErrFmtCannotModulo   = "cannot modulo %s"
	ErrFmtCannotNegate   = "cannot negate %s"
	ErrFmtCannotCompare  = "cannot compare %s"
	ErrFmtNotIterable    = "%s is not iterable"
	ErrFmtNotIndexable   = "%s is not indexable"
	// ErrFmtNotImplemented is raised by `Trait.method(x)` for an x without an
	// impl of the trait: the type name, then the trait name.
	ErrFmtNotImplemented = "%s does not implement %s"
	// ErrFmtMustReturn is raised when a trait method returns the wrong type:
	// the receiver type, the method, the expected and the actual type names.
	ErrFmtMustReturn = "%s.%s must return %s, got %s"
	// ErrFmtTraitNotDefined and ErrFmtNotATrait report a trait reference that
	// does not resolve: where it appears ("impl Shape", "trait Titled"), then
	// the reference.
	ErrFmtTraitNotDefined = "%s: %s is not defined"
	ErrFmtNotATrait       = "%s: %s is not a trait"
)

// unsupported marks a TypeError saying that a type has no form of an
// operation at all, as opposed to not accepting this particular operand:
// Function has no + whatever the other side is, while Integer has one that a
// String does not fit. The operators read it as any other TypeError. A trait
// method called on a built-in value reports it as the type not implementing
// the trait instead (see Trait.call).
type unsupported struct{ typeName string }

func (u unsupported) Error() string { return u.typeName + " does not support the operation" }

// NewUnsupportedError builds the TypeError an Object method raises for an
// operation its type never supports. typeName is the receiver's TypeName.
func NewUnsupportedError(typeName, format string, a ...any) *Error {
	return &Error{Value: fmt.Sprintf(format, a...), Wrapped: typedCause{cause: unsupported{typeName}, base: TypeError}}
}

// Unsupported is NewUnsupportedError for the usual case of a format whose
// only argument is the type name, e.g. Unsupported("Nil", ErrFmtCannotAdd).
func Unsupported(typeName, format string) *Error {
	return NewUnsupportedError(typeName, format, typeName)
}

// Operation verbs for NewOperandError.
const (
	OpAdd      = "add"
	OpSubtract = "subtract"
	OpMultiply = "multiply"
	OpDivide   = "divide"
	OpModulo   = "modulo"
	OpCompare  = "compare"
)

// NewOperandError is the TypeError for an operation the left operand's type
// supports, but not with this right operand: "cannot add Integer and String".
func NewOperandError(op, leftType string, right Object) *Error {
	return NewTypeError("cannot %s %s and %s", op, leftType, right.TypeName())
}

// typeNamer names a type at the type level, so the zero-size inert mixin can
// word its errors without storing the name.
type typeNamer interface{ typeName() string }

// inert supplies the operator methods of a built-in type that has none of
// them: each raises the unsupported error. A type embeds inert[itsName] and
// overrides whatever it does support.
type inert[N typeNamer] struct {
	NoReflectedOps
	NoAssignment
}

func inertError[N typeNamer](format string) *Error {
	var n N
	return Unsupported(n.typeName(), format)
}

func (inert[N]) Compare(Object) (int, error)     { return 0, inertError[N](ErrFmtCannotCompare) }
func (inert[N]) Add(Object) (Object, error)      { return nil, inertError[N](ErrFmtCannotAdd) }
func (inert[N]) Minus(Object) (Object, error)    { return nil, inertError[N](ErrFmtCannotSubtract) }
func (inert[N]) Multiply(Object) (Object, error) { return nil, inertError[N](ErrFmtCannotMultiply) }
func (inert[N]) Divide(Object) (Object, error)   { return nil, inertError[N](ErrFmtCannotDivide) }
func (inert[N]) Modulo(Object) (Object, error)   { return nil, inertError[N](ErrFmtCannotModulo) }
func (inert[N]) Iter() ([]Object, error)         { return nil, inertError[N](ErrFmtNotIterable) }
func (inert[N]) Index(Object) (Object, error)    { return nil, inertError[N](ErrFmtNotIndexable) }

// isUnsupported reports whether err says that recv's type lacks the
// operation entirely.
func isUnsupported(err error, recv Object) bool {
	var u unsupported
	return errors.As(err, &u) && u.typeName == recv.TypeName()
}
