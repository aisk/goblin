package object

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
