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

// mirror is the operator asking the same question with the operands swapped.
func (op orderOp) mirror() orderOp {
	switch op {
	case opLt:
		return opGt
	case opLe:
		return opGe
	case opGt:
		return opLt
	}
	return opLe
}

// Less, LessEqual, Greater and GreaterEqual are the entry points both backends
// use for <, <=, > and >=. They differ from Compare in one way: a user value
// answers through its Ord impl's lt, le, gt or ge, so an impl that overrides
// one of those is what the operator runs. Dispatch is symmetric like Compare:
// when the left operand has no ordering for the right one, the right operand
// answers the mirrored question.
func Less(a, b Object) (bool, error) {
	if c, ok := fastCompare(a, b); ok {
		return c < 0, nil
	}
	return order(a, b, opLt)
}

func LessEqual(a, b Object) (bool, error) {
	if c, ok := fastCompare(a, b); ok {
		return c <= 0, nil
	}
	return order(a, b, opLe)
}

func Greater(a, b Object) (bool, error) {
	if c, ok := fastCompare(a, b); ok {
		return c > 0, nil
	}
	return order(a, b, opGt)
}

func GreaterEqual(a, b Object) (bool, error) {
	if c, ok := fastCompare(a, b); ok {
		return c >= 0, nil
	}
	return order(a, b, opGe)
}

// fastCompare orders numbers and strings without an interface call.
func fastCompare(a, b Object) (int, bool) {
	if c, ok := numericCompare(a, b); ok {
		return c, true
	}
	if lhs, ok := a.(String); ok {
		if rhs, ok := b.(String); ok {
			return compareOrdered(lhs, rhs), true
		}
	}
	return 0, false
}

func order(a, b Object, op orderOp) (bool, error) {
	r, err := orderSide(a, b, op)
	if err == nil {
		return r, nil
	}
	if !notHandled(err) {
		return false, err
	}
	r, rerr := orderSide(b, a, op.mirror())
	if rerr == nil {
		return r, nil
	}
	if !notHandled(rerr) {
		return false, rerr
	}
	return false, err
}

func orderSide(a, b Object, op orderOp) (bool, error) {
	if uv, ok := a.(UserValue); ok {
		return userOrder(uv, op, b)
	}
	c, err := a.Compare(b)
	if err != nil {
		return false, err
	}
	return op.test(c), nil
}
