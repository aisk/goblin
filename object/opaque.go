package object

// OpaqueBase provides the boilerplate Object methods shared by opaque handle
// types (files, HTTP responses, compiled patterns, UUIDs, ...): always
// truthy, no structural equality (object.Equals' identity backstop still
// applies), and no arithmetic, comparison, iteration, or indexing. A concrete
// type embeds MakeOpaqueBase("Name") and implements String(), ToString(),
// GetAttr() and Attributes(), overriding any default it refines.
type OpaqueBase struct {
	NoReflectedOps
	NoAssignment
	typeName string
}

// MakeOpaqueBase returns the embeddable base for an opaque type with the
// given Goblin-level type name.
func MakeOpaqueBase(typeName string) OpaqueBase {
	return OpaqueBase{typeName: typeName}
}

func (b OpaqueBase) TypeName() string { return b.typeName }

func (b OpaqueBase) ToBool() (bool, error) { return true, nil }

func (b OpaqueBase) Equals(Object) (bool, error) { return false, nil }

func (b OpaqueBase) Compare(Object) (int, error) {
	return 0, Unsupported(b.typeName, ErrFmtCannotCompare)
}

func (b OpaqueBase) Add(Object) (Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtCannotAdd)
}

func (b OpaqueBase) Minus(Object) (Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtCannotSubtract)
}

func (b OpaqueBase) Multiply(Object) (Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtCannotMultiply)
}

func (b OpaqueBase) Divide(Object) (Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtCannotDivide)
}

func (b OpaqueBase) Modulo(Object) (Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtCannotModulo)
}

func (b OpaqueBase) Iter() ([]Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtNotIterable)
}

func (b OpaqueBase) Index(Object) (Object, error) {
	return nil, Unsupported(b.typeName, ErrFmtNotIndexable)
}
