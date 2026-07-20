package object

var Nil Object = Unit{}

type Unit struct{}

var _ Object = Unit{}

func (n Unit) String() string {
	return "none"
}

func (n Unit) ToString() (string, error) { return n.String(), nil }

func (n Unit) Bool() bool {
	return false
}

func (n Unit) ToBool() (bool, error) { return n.Bool(), nil }

func (n Unit) Compare(other Object) (int, error) {
	switch other.(type) {
	case Unit:
		return 0, nil
	default:
		return 0, NewTypeError("cannot compare Nil and %T", other)
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

func (n Unit) Not() (Object, error) {
	return Bool(!n.Bool()), nil
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
