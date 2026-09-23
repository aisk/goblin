package object

import (
	"errors"
	"fmt"
)

// Diagnostics raised by operator and trait dispatch. Both backends format them
// with the type's declared name, so an error reads identically whichever
// backend runs the program.
const (
	ErrFmtCannotAdd      = "cannot add %s"
	ErrFmtCannotSubtract = "cannot subtract %s"
	ErrFmtCannotMultiply = "cannot multiply %s"
	ErrFmtCannotDivide   = "cannot divide %s"
	ErrFmtCannotModulo   = "cannot modulo %s"
	ErrFmtCannotNegate   = "cannot negate %s"
	ErrFmtCannotCompare  = "cannot compare %s"
	ErrFmtNotIterable    = "%s does not support iteration"
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

// isUnsupported reports whether err says that recv's type lacks the
// operation entirely.
func isUnsupported(err error, recv Object) bool {
	var u unsupported
	return errors.As(err, &u) && u.typeName == recv.TypeName()
}
