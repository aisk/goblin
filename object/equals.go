package object

import "reflect"

// Equals reports whether two objects are equal under Goblin's == operator.
// Equality stays total across types: values of unrelated types are simply
// unequal, and a TypeError raised while comparing them says exactly that.
// The error channel is for a user type's Eq impl that fails for a reason of
// its own (a division by zero, a raise of its own), which must not be
// reported as "not equal".
//
// Numbers and strings are answered without the interface call the general path
// pays for; the result is the one Integer.Equals, Float.Equals and
// String.Equals define.
func Equals(a, b Object) (bool, error) {
	if eq, ok := numericEquals(a, b); ok {
		return eq, nil
	}
	if lhs, ok := a.(String); ok {
		if rhs, ok := b.(String); ok {
			return lhs == rhs, nil
		}
	}
	return equalsOthers(a, b)
}

// equalsOthers is Equals for a pair that is not two numbers or two strings.
//
//   - nil equals only nil, without asking the other operand, so `x == nil`
//     is a safe test even for an eq that reads fields of other.
//   - With a user value on the left: the same object is equal to itself
//     without its eq being asked. Otherwise its eq decides when it answers;
//     only when the left has no eq for the pair (no impl Eq, or its eq raised
//     TypeError) is the right operand asked, the rule compare follows.
//   - With a built-in on the left, either operand may recognize the other
//     (this is how `1 == n` reaches a user type's Eq impl on n), with a final
//     identity backstop for types whose Equals has no equality of its own.
func equalsOthers(a, b Object) (bool, error) {
	if _, ok := a.(Unit); ok {
		_, ok = b.(Unit)
		return ok, nil
	}
	if _, ok := b.(Unit); ok {
		return false, nil
	}
	if u, ok := a.(UserValue); ok {
		if Object(u) == b {
			return true, nil
		}
		if impl := u.UserType().impls[eqID]; impl != nil {
			r, err := impl.call(u, EqEq, []Object{u, b})
			if err == nil {
				return bool(r.(Bool)), nil
			}
			if !notHandled(err) {
				return false, err
			}
		}
		return equalsSide(b, a)
	}
	eq, err := equalsSide(a, b)
	if err != nil || eq {
		return eq, err
	}
	if eq, err = equalsSide(b, a); err != nil || eq {
		return eq, err
	}
	return identical(a, b), nil
}

// NotEquals is the != operator: the negation of Equals.
func NotEquals(a, b Object) (bool, error) {
	eq, err := Equals(a, b)
	return !eq, err
}

// equalsSide asks one operand about the other. A TypeError means "I do not
// know this type", which is the unequal answer rather than a failure — that is
// what keeps `x == nil` safe for an eq written only for real operands. Any
// other error is a genuine failure inside the comparison and propagates.
func equalsSide(a, b Object) (bool, error) {
	eq, err := a.Equals(b)
	if err != nil {
		if notHandled(err) {
			return false, nil
		}
		return false, err
	}
	return eq, nil
}

// identical reports whether two objects are the same value. It guards against
// uncomparable underlying types (e.g. slice-backed Bytes), for which interface
// equality would panic.
func identical(a, b Object) bool {
	ta := reflect.TypeOf(a)
	if ta != reflect.TypeOf(b) || !ta.Comparable() {
		return false
	}
	return a == b
}
