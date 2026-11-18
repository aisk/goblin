package object

import "math"

// The numeric helpers below hold Goblin's rules for the Integer/Float pairs in
// one place: an all-integer expression stays integer, a mixed one widens to
// float, division or modulo by either kind of zero raises
// ZeroDivisionError, and integer results that do not fit raise OverflowError.
// Both
// the operand types' own methods and the package-level operators call them —
// the operators would otherwise reach the very same code through an interface
// call, which is measurable in arithmetic-heavy loops.
//
// The bool reports whether the pair was numeric at all. When it is false the
// caller raises the type error itself, so each caller keeps naming the types
// the way it sees them.

func numericAdd(a, b Object) (Object, bool, error) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			c, ok := CheckedAdd(int64(lhs), int64(rhs))
			if !ok {
				return nil, true, NewIntegerOverflow()
			}
			return Integer(c), true, nil
		case Float:
			return Float(lhs) + rhs, true, nil
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return lhs + rhs, true, nil
		case Integer:
			return lhs + Float(rhs), true, nil
		}
	}
	return nil, false, nil
}

func numericMinus(a, b Object) (Object, bool, error) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			c, ok := CheckedSub(int64(lhs), int64(rhs))
			if !ok {
				return nil, true, NewIntegerOverflow()
			}
			return Integer(c), true, nil
		case Float:
			return Float(lhs) - rhs, true, nil
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return lhs - rhs, true, nil
		case Integer:
			return lhs - Float(rhs), true, nil
		}
	}
	return nil, false, nil
}

func numericMultiply(a, b Object) (Object, bool, error) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			c, ok := CheckedMul(int64(lhs), int64(rhs))
			if !ok {
				return nil, true, NewIntegerOverflow()
			}
			return Integer(c), true, nil
		case Float:
			return Float(lhs) * rhs, true, nil
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return lhs * rhs, true, nil
		case Integer:
			return lhs * Float(rhs), true, nil
		}
	}
	return nil, false, nil
}

func numericDivide(a, b Object) (Object, bool, error) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("division by zero")
			}
			c, ok := CheckedDiv(int64(lhs), int64(rhs))
			if !ok {
				return nil, true, NewIntegerOverflow()
			}
			return Integer(c), true, nil
		case Float:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("division by zero")
			}
			return Float(lhs) / rhs, true, nil
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("division by zero")
			}
			return lhs / rhs, true, nil
		case Integer:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("division by zero")
			}
			return lhs / Float(rhs), true, nil
		}
	}
	return nil, false, nil
}

func numericModulo(a, b Object) (Object, bool, error) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("modulo by zero")
			}
			return lhs % rhs, true, nil
		case Float:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("modulo by zero")
			}
			return Float(math.Mod(float64(lhs), float64(rhs))), true, nil
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("modulo by zero")
			}
			return Float(math.Mod(float64(lhs), float64(rhs))), true, nil
		case Integer:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("modulo by zero")
			}
			return Float(math.Mod(float64(lhs), float64(rhs))), true, nil
		}
	}
	return nil, false, nil
}

func numericEquals(a, b Object) (bool, bool) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			return lhs == rhs, true
		case Float:
			return Float(lhs) == rhs, true
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			// IEEE 754 semantics: NaN equals nothing, including itself.
			return lhs == rhs, true
		case Integer:
			return lhs == Float(rhs), true
		}
	}
	return false, false
}

// numericCompare ranks two numbers, reporting false when either operand is
// not a number. A pair with a NaN has no order and ranks as unordered.
func numericCompare(a, b Object) (int, bool) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			return compareOrdered(lhs, rhs), true
		case Float:
			return compareFloats(Float(lhs), rhs), true
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return compareFloats(lhs, rhs), true
		case Integer:
			return compareFloats(lhs, Float(rhs)), true
		}
	}
	return 0, false
}

// unordered is the rank of a pair of numbers with a NaN among them. It is
// outside -1, 0 and 1, so `c < 0`, `c <= 0` and `c == 1` are all false for
// it, which is how the ordering operators answer false for NaN (IEEE 754)
// without a test of their own; Compare reports it as errUnordered.
const unordered = 2

// compareOrdered ranks two values of the same totally ordered type.
func compareOrdered[T Integer | String](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compareFloats ranks two floats, unordered when either is NaN.
func compareFloats(a, b Float) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	case a == b:
		return 0
	}
	return unordered
}

// errUnordered is what ordering a NaN raises anywhere but in the four
// ordering operators: compare(), sorting, max and min have no answer to give.
func errUnordered() error { return NewValueError("cannot order NaN") }
