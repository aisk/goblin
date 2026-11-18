package object

// Compare orders two objects under Goblin's <, <=, > and >= operators.
// Dispatch is symmetric in the same way as Equals: when the left operand has
// no ordering for the right one, the right operand is asked to order itself
// against the left and the result is negated. This is how `1 < n` reaches a
// user type's Ord impl on n, matching the `1 == n` that Equals already allows.
//
// Only a TypeError from the left operand triggers the reflected attempt; any
// other failure (a division by zero raised inside an Ord impl, for example)
// propagates untouched. When neither side can order the pair, the left
// operand's error is the one reported, since it names the expression's own
// operand order.
//
// Numbers and strings, which every loop condition is made of, are ordered here
// without the interface call the general path pays for; the result is the one
// Integer.Compare, Float.Compare and String.Compare define.
//
// A NaN has no place in an order, so Compare raises ValueError for it: every
// caller (compare(), sorting, max, min, list and structural ordering) would
// otherwise give an answer that is not one. The four ordering operators do
// not go through this: they answer false for a NaN operand.
func Compare(a, b Object) (int, error) {
	if c, ok := numericCompare(a, b); ok {
		if c == unordered {
			return 0, errUnordered()
		}
		return c, nil
	}
	return compareOthers(a, b)
}

// compareOthers is Compare for a pair that is not two numbers.
func compareOthers(a, b Object) (int, error) {
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
	if !notHandled(err) {
		return 0, err
	}
	reflected, rerr := b.Compare(a)
	if rerr == nil {
		return -reflected, nil
	}
	if !notHandled(rerr) {
		return 0, rerr
	}
	return 0, err
}

// Less, LessEqual, Greater and GreaterEqual are the entry points both backends
// use for <, <=, > and >=. They answer from the same three-way comparison as
// Compare, so a user value orders through its Ord impl's compare, the method
// sorting uses. Two numbers are ranked directly: a pair with a NaN ranks
// unordered, for which each test below is false, as IEEE 754 has it, and
// which the native float comparisons of compiled code agree with.
func Less(a, b Object) (bool, error) {
	if c, ok := numericCompare(a, b); ok {
		return c < 0, nil
	}
	c, err := compareOthers(a, b)
	return err == nil && c < 0, err
}

func LessEqual(a, b Object) (bool, error) {
	if c, ok := numericCompare(a, b); ok {
		return c <= 0, nil
	}
	c, err := compareOthers(a, b)
	return err == nil && c <= 0, err
}

func Greater(a, b Object) (bool, error) {
	if c, ok := numericCompare(a, b); ok {
		return c == 1, nil
	}
	c, err := compareOthers(a, b)
	return err == nil && c > 0, err
}

func GreaterEqual(a, b Object) (bool, error) {
	if c, ok := numericCompare(a, b); ok {
		return c == 0 || c == 1, nil
	}
	c, err := compareOthers(a, b)
	return err == nil && c >= 0, err
}
