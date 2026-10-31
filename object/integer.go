package object

import (
	"errors"
	"math"
	"strconv"
)

type Integer int64

var _ Object = Integer(0)

func (i Integer) ToBool() (bool, error) { return i != 0, nil }

func (i Integer) TypeName() string { return "Integer" }

func (i Integer) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Integer) ToString() (string, error) { return i.String(), nil }

func (i Integer) Equals(other Object) (bool, error) {
	eq, _ := numericEquals(i, other)
	return eq, nil
}

func (i Integer) Compare(other Object) (int, error) {
	if c, ok := numericCompare(i, other); ok {
		return c, nil
	}
	return 0, NewOperandError(OpCompare, "Integer", other)
}

func (i Integer) Add(other Object) (Object, error) {
	if result, ok, err := numericAdd(i, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpAdd, "Integer", other)
}

func (i Integer) Minus(other Object) (Object, error) {
	if result, ok, err := numericMinus(i, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpSubtract, "Integer", other)
}

func (i Integer) Multiply(other Object) (Object, error) {
	if result, ok, err := numericMultiply(i, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpMultiply, "Integer", other)
}

func (i Integer) Divide(other Object) (Object, error) {
	if result, ok, err := numericDivide(i, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpDivide, "Integer", other)
}

func (i Integer) Modulo(other Object) (Object, error) {
	if result, ok, err := numericModulo(i, other); ok {
		return result, err
	}
	return nil, NewOperandError(OpModulo, "Integer", other)
}

// A named int64 type cannot embed NoAssignment and NoReflectedOps, so Integer
// declines assignment and the reflected operators itself.
func (i Integer) SetIndex(Object, Object) (bool, error)  { return false, nil }
func (i Integer) SetAttr(string, Object) (bool, error)   { return false, nil }
func (i Integer) RAdd(Object) (Object, bool, error)      { return nil, false, nil }
func (i Integer) RMinus(Object) (Object, bool, error)    { return nil, false, nil }
func (i Integer) RMultiply(Object) (Object, bool, error) { return nil, false, nil }
func (i Integer) RDivide(Object) (Object, bool, error)   { return nil, false, nil }
func (i Integer) RModulo(Object) (Object, bool, error)   { return nil, false, nil }

func (i Integer) Iter() ([]Object, error) {
	return nil, Unsupported("Integer", ErrFmtNotIterable)
}

func (i Integer) Index(index Object) (Object, error) {
	return nil, Unsupported("Integer", ErrFmtNotIndexable)
}

func (i Integer) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(i), nil
	case "constructor":
		return IntConstructorFn, nil
	default:
		return nil, NewAttributeError("Integer has no attribute '%s'", name)
	}
}

func (i Integer) Attributes() []string { return []string{"attributes", "constructor"} }

var IntConstructorFn = &Function{Name: "Int", Fn: IntConstructor}

func IntConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Int", args)
	value := ap.AnyOr("value", Integer(0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case Integer:
		return v, nil
	case Float:
		f := float64(v)
		if math.IsNaN(f) {
			return nil, NewValueError("Int() cannot convert NaN")
		}
		// -2^63 and 2^63 are exact float64 values; the truncation of anything
		// in [-2^63, 2^63) fits in an int64.
		if f < math.MinInt64 || f >= -math.MinInt64 {
			return nil, NewOverflowError("Int() value out of range: %s", v.String())
		}
		return Integer(int64(f)), nil
	case String:
		n, err := strconv.ParseInt(string(v), 10, 64)
		if errors.Is(err, strconv.ErrRange) {
			return nil, NewOverflowError("Int() value out of range: %q", string(v))
		}
		if err != nil {
			return nil, NewValueError("Int() invalid literal for Int: %q", string(v))
		}
		return Integer(n), nil
	case Bool:
		if bool(v) {
			return Integer(1), nil
		}
		return Integer(0), nil
	default:
		return nil, NewTypeError("Int() argument 'value' must be String, Integer or Float, got %s", value.TypeName())
	}
}
