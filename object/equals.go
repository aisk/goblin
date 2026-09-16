package object

import "reflect"

// Equals reports whether two objects are equal under Goblin's == operator.
// Equality stays total across types: values of unrelated types are simply
// unequal, and a TypeError raised while comparing them says exactly that, so
// `x == nil` never fails. The error channel is for a user type's Eq impl that
// fails for a reason of its own — a division by zero, a raise of its own —
// which must not be reported as "not equal".
//
// Dispatch is symmetric — either operand's Equals may recognize the other
// (this is how `1 == n` reaches a user type's Eq impl on n) — with a final
// identity backstop for types whose Equals has no natural equality of its own.
//
// Numbers and strings are answered without the interface call the general path
// pays for; the result is the one Integer.Equals, Float.Equals and
// String.Equals define.
func Equals(a, b Object) (bool, error) {
	return equality(a, b, false)
}

// NotEquals is the != operator: the negation of the same symmetric walk
// Equals makes, except that a user value whose Eq impl overrides ne answers
// its side through that method.
func NotEquals(a, b Object) (bool, error) {
	eq, err := equality(a, b, true)
	return !eq, err
}

func equality(a, b Object, viaNe bool) (bool, error) {
	if eq, ok := numericEquals(a, b); ok {
		return eq, nil
	}
	if lhs, ok := a.(String); ok {
		if rhs, ok := b.(String); ok {
			return lhs == rhs, nil
		}
	}
	eq, err := equalsSide(a, b, viaNe)
	if err != nil || eq {
		return eq, err
	}
	if eq, err = equalsSide(b, a, viaNe); err != nil || eq {
		return eq, err
	}
	return identical(a, b), nil
}

// equalsSide asks one operand about the other. A TypeError means "I do not
// know this type", which is the unequal answer rather than a failure — that is
// what keeps `x == nil` safe for an eq written only for real operands. Any
// other error is a genuine failure inside the comparison and propagates.
func equalsSide(a, b Object, viaNe bool) (bool, error) {
	var eq bool
	var err error
	if ne, ok := overriddenNe(a); viaNe && ok {
		var r Object
		if r, err = ne.call(a, EqNe, []Object{a, b}); err == nil {
			eq = !bool(r.(Bool))
		}
	} else {
		eq, err = a.Equals(b)
	}
	if err != nil {
		if notHandled(err) {
			return false, nil
		}
		return false, err
	}
	return eq, nil
}

// overriddenNe returns the Eq impl of a user value that defines ne itself.
func overriddenNe(v Object) (*TraitImpl, bool) {
	uv, ok := v.(UserValue)
	if !ok {
		return nil, false
	}
	impl := uv.UserType().eq
	return impl, impl != nil && impl.supplies(EqNe)
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
