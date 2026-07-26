package object

import "errors"

// Compare orders two objects under Goblin's <, <=, > and >= operators.
// Dispatch is symmetric in the same way as Equals: when the left operand has
// no ordering for the right one, the right operand is asked to order itself
// against the left and the result is negated. This is how `1 < n` reaches a
// user-defined __cmp on n, matching the `1 == n` that Equals already allows.
//
// Only a TypeError from the left operand triggers the reflected attempt; any
// other failure (a division by zero raised inside a user __cmp, for example)
// propagates untouched. When neither side can order the pair, the left
// operand's error is the one reported, since it names the expression's own
// operand order.
//
// Numbers and strings, which every loop condition is made of, are ordered here
// without the interface call the general path pays for; the result is the one
// Integer.Compare, Float.Compare and String.Compare define.
func Compare(a, b Object) (int, error) {
	if c, ok := numericCompare(a, b); ok {
		return c, nil
	}
	if lhs, ok := a.(String); ok {
		if rhs, ok := b.(String); ok {
			return compareOrdered(lhs, rhs), nil
		}
	}
	c, err := a.Compare(b)
	if err == nil {
		return c, nil
	}
	return compareReflected(a, b, err)
}

// compareReflected gives the right operand its turn once the left one reported
// that it has no ordering for the pair.
func compareReflected(a, b Object, err error) (int, error) {
	if !errors.Is(err, TypeError) {
		return 0, err
	}
	if reflected, rerr := b.Compare(a); rerr == nil {
		return -reflected, nil
	}
	return 0, err
}
