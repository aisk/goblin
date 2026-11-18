package object

// Messages about names, shared by the semantic checker, which reports them
// before a file runs, and the interpreter, which raises them as NameError
// for REPL input the checker never saw.
const (
	// ErrFmtUndefined is reading a name nothing declares.
	ErrFmtUndefined = "undefined identifier: %s"
	// ErrFmtAssignUndefined is assigning to a name nothing declares; a new
	// binding needs var.
	ErrFmtAssignUndefined = "assignment to undefined identifier: %s"
	// ErrFmtAssignBuiltin is assigning to a built-in name the program has
	// not declared itself. The built-in stays; var declares a binding that
	// shadows it.
	ErrFmtAssignBuiltin = "cannot assign to built-in '%s' (declare your own with var)"
	// ErrFmtAssignMethod is `p.m = value` where m is a method of p's type,
	// with the method and the type name: an instance stores only fields.
	ErrFmtAssignMethod = "cannot assign to method '%s' of %s"
)
