package object

var Nil Object = Unit{}

type Unit struct {
	inert[nilName]
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
		return 0, NewOperandError(OpCompare, "Nil", other)
	}
}

func (n Unit) GetAttr(name string) (Object, error) {
	if name == "attributes" {
		return AttributesFunction(n), nil
	}
	return nil, NewAttributeError("Nil has no attribute '%s'", name)
}

func (n Unit) Attributes() []string { return []string{"attributes"} }

type nilName struct{}

func (nilName) typeName() string { return "Nil" }
