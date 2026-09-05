package object

// Operators with a native integer on the right.
//
// The transpiler keeps provably-integer locals as Go int64; when such a value
// meets a boxed operand (a list element, a parameter, a field) the general
// entry points would first box it — an allocation for anything above 255 —
// and then dispatch on both sides. These variants take the integer as it is
// and answer the numeric cases directly. Every other receiver reaches exactly
// the boxed call it would have reached anyway, so a user type's __add or
// __cmp sees the same Integer operand and produces the same error text.
//
// Only the boxed-left, native-right shape has a variant: the operators are
// not symmetric (`a - b`, and which side's error is reported when neither
// side handles the pair), and that shape — an accumulator or element on the
// left, a counter or constant on the right — is the one hot code is made of.

func AddInt(a Object, b int64) (Object, error) {
	switch lhs := a.(type) {
	case Integer:
		return lhs + Integer(b), nil
	case Float:
		return lhs + Float(b), nil
	}
	return Add(a, Integer(b))
}

func MinusInt(a Object, b int64) (Object, error) {
	switch lhs := a.(type) {
	case Integer:
		return lhs - Integer(b), nil
	case Float:
		return lhs - Float(b), nil
	}
	return Minus(a, Integer(b))
}

func MultiplyInt(a Object, b int64) (Object, error) {
	switch lhs := a.(type) {
	case Integer:
		return lhs * Integer(b), nil
	case Float:
		return lhs * Float(b), nil
	}
	return Multiply(a, Integer(b))
}

// DivideInt and ModuloInt keep the zero check on the general path: it is
// exactly the error the boxed call raises.
func DivideInt(a Object, b int64) (Object, error) {
	if b != 0 {
		switch lhs := a.(type) {
		case Integer:
			return lhs / Integer(b), nil
		case Float:
			return lhs / Float(b), nil
		}
	}
	return Divide(a, Integer(b))
}

func ModuloInt(a Object, b int64) (Object, error) {
	if b != 0 {
		if lhs, ok := a.(Integer); ok {
			return lhs % Integer(b), nil
		}
	}
	return Modulo(a, Integer(b))
}

func CompareInt(a Object, b int64) (int, error) {
	if lhs, ok := a.(Integer); ok {
		return compareOrdered(lhs, Integer(b)), nil
	}
	return Compare(a, Integer(b))
}

// EqualsInt is symmetric like Equals, so the transpiler may use it for the
// native-left shape as well.
func EqualsInt(a Object, b int64) (bool, error) {
	if lhs, ok := a.(Integer); ok {
		return int64(lhs) == b, nil
	}
	return Equals(a, Integer(b))
}
