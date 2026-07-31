package object

import "math"

// The numeric helpers below hold Goblin's rules for the Integer/Float pairs in
// one place: an all-integer expression stays integer, a mixed one widens to
// float, and division or modulo by either kind of zero raises
// ZeroDivisionError. Both
// the operand types' own methods and the package-level operators call them —
// the operators would otherwise reach the very same code through an interface
// call, which is measurable in arithmetic-heavy loops.
//
// The bool reports whether the pair was numeric at all. When it is false the
// caller raises the type error itself, so each caller keeps naming the types
// the way it sees them.

func numericAdd(a, b Object) (Object, bool) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			return lhs + rhs, true
		case Float:
			return Float(lhs) + rhs, true
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return lhs + rhs, true
		case Integer:
			return lhs + Float(rhs), true
		}
	}
	return nil, false
}

func numericMinus(a, b Object) (Object, bool) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			return lhs - rhs, true
		case Float:
			return Float(lhs) - rhs, true
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return lhs - rhs, true
		case Integer:
			return lhs - Float(rhs), true
		}
	}
	return nil, false
}

func numericMultiply(a, b Object) (Object, bool) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			return lhs * rhs, true
		case Float:
			return Float(lhs) * rhs, true
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return lhs * rhs, true
		case Integer:
			return lhs * Float(rhs), true
		}
	}
	return nil, false
}

func numericDivide(a, b Object) (Object, bool, error) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			if rhs == 0 {
				return nil, true, NewZeroDivisionError("division by zero")
			}
			return lhs / rhs, true, nil
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

func numericCompare(a, b Object) (int, bool) {
	switch lhs := a.(type) {
	case Integer:
		switch rhs := b.(type) {
		case Integer:
			return compareOrdered(lhs, rhs), true
		case Float:
			return compareOrdered(Float(lhs), rhs), true
		}
	case Float:
		switch rhs := b.(type) {
		case Float:
			return compareOrdered(lhs, rhs), true
		case Integer:
			return compareOrdered(lhs, Float(rhs)), true
		}
	}
	return 0, false
}

// compareOrdered ranks two values of the same ordered type. Floats follow IEEE
// 754 through the < and > operators: a NaN operand is neither, and reports as
// equal, which is what Float.Compare has always done.
func compareOrdered[T Integer | Float | String](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
