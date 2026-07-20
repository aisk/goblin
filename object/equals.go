package object

import "reflect"

// Equals reports whether two objects are equal under Goblin's == operator.
// Unlike Compare, equality is total: values of unrelated types are simply
// unequal, so == never raises. Core built-in types get structural equality;
// other types are consulted through Compare (which dispatches a user-defined
// __cmp on either operand) and finally fall back to identity.
func Equals(a, b Object) bool {
	if eq, ok := coreEquals(a, b); ok {
		return eq
	}
	if c, err := a.Compare(b); err == nil {
		return c == 0
	}
	if c, err := b.Compare(a); err == nil {
		return c == 0
	}
	return identical(a, b)
}

// isCore reports whether an object is one of the core built-in types whose
// equality is defined structurally by coreEquals. Types outside this set
// (Bytes, Path, user-defined instances, ...) rely on Compare or identity.
func isCore(o Object) bool {
	switch o.(type) {
	case Integer, Float, Bool, String, Unit, *List, *Dict, *Function:
		return true
	}
	return false
}

// coreEquals implements structural equality between core built-in types. ok is
// false when either operand is not a core type, in which case the caller must
// fall back to Compare dispatch so user-defined __cmp still gets a say.
func coreEquals(a, b Object) (eq bool, ok bool) {
	if !isCore(a) || !isCore(b) {
		return false, false
	}
	switch av := a.(type) {
	case Integer:
		switch bv := b.(type) {
		case Integer:
			return av == bv, true
		case Float:
			return float64(av) == float64(bv), true
		}
	case Float:
		switch bv := b.(type) {
		case Integer:
			return float64(av) == float64(bv), true
		case Float:
			// IEEE 754 semantics: NaN is not equal to anything, including itself.
			return av == bv, true
		}
	case Bool:
		if bv, isBool := b.(Bool); isBool {
			return av == bv, true
		}
	case String:
		if bv, isStr := b.(String); isStr {
			return av == bv, true
		}
	case Unit:
		_, isUnit := b.(Unit)
		return isUnit, true
	case *List:
		if bv, isList := b.(*List); isList {
			if len(av.Elements) != len(bv.Elements) {
				return false, true
			}
			for i, elem := range av.Elements {
				if !Equals(elem, bv.Elements[i]) {
					return false, true
				}
			}
			return true, true
		}
	case *Dict:
		if bv, isDict := b.(*Dict); isDict {
			if len(av.Entries) != len(bv.Entries) {
				return false, true
			}
			for key, entry := range av.Entries {
				other, exists := bv.Entries[key]
				if !exists || !Equals(entry.Value, other.Value) {
					return false, true
				}
			}
			return true, true
		}
	case *Function:
		if bv, isFn := b.(*Function); isFn {
			return av == bv, true
		}
	}
	// Both are core types but of unrelated kinds (e.g. Integer vs String).
	return false, true
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
