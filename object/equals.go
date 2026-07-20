package object

import "reflect"

// Equals reports whether two objects are equal under Goblin's == operator.
// Unlike Compare, equality is total: values of unrelated types are simply
// unequal, so == never raises. Dispatch is symmetric — either operand's
// Equals method may recognize the other (this is how `1 == n` reaches a
// user-defined __cmp on n) — with a final identity backstop for types whose
// Equals has no natural equality of its own.
func Equals(a, b Object) bool {
	if a.Equals(b) || b.Equals(a) {
		return true
	}
	return identical(a, b)
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
