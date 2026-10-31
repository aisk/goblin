package object

import (
	"strconv"
)

type Float float64

var _ Object = Float(0)

func (f Float) ToBool() (bool, error) { return f != 0, nil }

func (f Float) TypeName() string { return "Float" }

func (f Float) String() string {
	return strconv.FormatFloat(float64(f), 'f', -1, 64)
}

func (f Float) ToString() (string, error) { return f.String(), nil }

func (f Float) Equals(other Object) (bool, error) {
	eq, _ := numericEquals(f, other)
	return eq, nil
}

func (f Float) Compare(other Object) (int, error) {
	if c, ok := numericCompare(f, other); ok {
		return c, nil
	}
	return 0, NewOperandError(OpCompare, "Float", other)
}

func (f Float) Add(other Object) (Object, error) {
	if result, ok := numericAdd(f, other); ok {
		return result, nil
	}
	return nil, NewOperandError(OpAdd, "Float", other)
}

func (f Float) Minus(other Object) (Object, error) {
	if result, ok := numericMinus(f, other); ok {
		return result, nil
	}
	return nil, NewOperandError(OpSubtract, "Float", other)
}

func (f Float) Multiply(other Object) (Object, error) {
	if result, ok := numericMultiply(f, other); ok {
		return result, nil
	}
	return nil, NewOperandError(OpMultiply, "Float", other)
}

func (f Float) Divide(other Object) (Object, error) {
	if result, ok, err := numericDivide(f, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpDivide, "Float", other)
}

func (f Float) Modulo(other Object) (Object, error) {
	if result, ok, err := numericModulo(f, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpModulo, "Float", other)
}

// A named float64 type cannot embed NoAssignment and NoReflectedOps, so Float
// declines assignment and the reflected operators itself.
func (f Float) SetIndex(Object, Object) (bool, error)  { return false, nil }
func (f Float) SetAttr(string, Object) (bool, error)   { return false, nil }
func (f Float) RAdd(Object) (Object, bool, error)      { return nil, false, nil }
func (f Float) RMinus(Object) (Object, bool, error)    { return nil, false, nil }
func (f Float) RMultiply(Object) (Object, bool, error) { return nil, false, nil }
func (f Float) RDivide(Object) (Object, bool, error)   { return nil, false, nil }
func (f Float) RModulo(Object) (Object, bool, error)   { return nil, false, nil }

func (f Float) Iter() ([]Object, error) {
	return nil, Unsupported("Float", ErrFmtNotIterable)
}

func (f Float) Index(index Object) (Object, error) {
	return nil, Unsupported("Float", ErrFmtNotIndexable)
}

func (f Float) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(f), nil
	case "constructor":
		return FloatConstructorFn, nil
	default:
		return nil, NewAttributeError("Float has no attribute '%s'", name)
	}
}

func (f Float) Attributes() []string { return []string{"attributes", "constructor"} }

var FloatConstructorFn = &Function{Name: "Float", Fn: FloatConstructor}

func FloatConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Float", args)
	value := ap.AnyOr("value", Float(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case Float:
		return v, nil
	case Integer:
		return Float(float64(int64(v))), nil
	case String:
		n, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return nil, NewValueError("Float() invalid literal for Float: %q", string(v))
		}
		return Float(n), nil
	case Bool:
		if bool(v) {
			return Float(1), nil
		}
		return Float(0), nil
	default:
		return nil, NewTypeError("Float() argument 'value' must be String, Integer or Float, got %s", value.TypeName())
	}
}
