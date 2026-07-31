package object

var Nil Object = Unit{}

type Unit struct {
	NoReflectedOps
	NoAssignment
}

var _ Object = Unit{}

func (n Unit) TypeName() string { return "Nil" }

func (n Unit) String() string {
	return "nil"
}

func (n Unit) ToString() (string, error) { return n.String(), nil }

func (n Unit) ToBool() (bool, error) { return false, nil }

func (n Unit) Equals(other Object) (bool, error) {
	_, ok := other.(Unit)
	return ok, nil
}

func (n Unit) Compare(other Object) (int, error) {
	switch other.(type) {
	case Unit:
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare Nil and %s", other.TypeName())
	}
}

func (n Unit) Add(other Object) (Object, error) {
	return nil, NewTypeError("cannot add to Nil")
}

func (n Unit) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from Nil")
}

func (n Unit) Multiply(other Object) (Object, error) {
	return nil, NewTypeError("cannot multiply Nil")
}

func (n Unit) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide Nil")
}

func (n Unit) Modulo(other Object) (Object, error) {
	return nil, NewTypeError("cannot modulo Nil")
}

func (n Unit) Not() (Object, error) {
	return True, nil
}

func (n Unit) Iter() ([]Object, error) {
	return nil, NewTypeError("Nil does not support iteration")
}

func (n Unit) Index(index Object) (Object, error) {
	return nil, NewTypeError("Nil is not indexable")
}

func (n Unit) GetAttr(name string) (Object, error) {
	if name == "attributes" {
		return AttributesFunction(n), nil
	}
	return nil, NewAttributeError("Nil has no attribute '%s'", name)
}

func (n Unit) Attributes() []string { return []string{"attributes"} }
