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

// orderOp is one of the four ordering operators. Its value is the offset of
// the matching Ord method from OrdLt.
type orderOp int

const (
	opLt orderOp = iota
	opLe
	opGt
	opGe
)

// test answers the operator from a three-way comparison result.
func (op orderOp) test(c int) bool {
	switch op {
	case opLt:
		return c < 0
	case opLe:
		return c <= 0
	case opGt:
		return c > 0
	}
	return c >= 0
}

// Less, LessEqual, Greater and GreaterEqual are the entry points both backends
// use for <, <=, > and >=. They answer from Compare, so a user value orders
// through its Ord impl's compare, the same method sorting uses.
func Less(a, b Object) (bool, error) {
	c, err := Compare(a, b)
	return err == nil && c < 0, err
}

func LessEqual(a, b Object) (bool, error) {
	c, err := Compare(a, b)
	return err == nil && c <= 0, err
}

func Greater(a, b Object) (bool, error) {
	c, err := Compare(a, b)
	return err == nil && c > 0, err
}

func GreaterEqual(a, b Object) (bool, error) {
	c, err := Compare(a, b)
	return err == nil && c >= 0, err
}
