package object

import (
	"errors"
	"reflect"
)

// Equals reports whether two objects are equal under Goblin's == operator.
// Equality stays total across types: values of unrelated types are simply
// unequal, and a TypeError raised while comparing them says exactly that, so
// `x == nil` never fails. The error channel is for a user-defined __cmp that
// fails for a reason of its own — a division by zero, a raise of its own —
// which must not be reported as "not equal".
//
// Dispatch is symmetric — either operand's Equals may recognize the other
// (this is how `1 == n` reaches a user-defined __cmp on n) — with a final
// identity backstop for types whose Equals has no natural equality of its own.
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
	eq, err := equalsSide(a, b)
	if err != nil || eq {
		return eq, err
	}
	if eq, err = equalsSide(b, a); err != nil || eq {
		return eq, err
	}
	return identical(a, b), nil
}

// equalsSide asks one operand about the other. A TypeError means "I do not
// know this type", which is the unequal answer rather than a failure — that is
// what keeps `x == nil` safe for a __cmp written only for real operands. Any
// other error is a genuine failure inside the comparison and propagates.
func equalsSide(a, b Object) (bool, error) {
	eq, err := a.Equals(b)
	if err != nil && !errors.Is(err, TypeError) {
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
