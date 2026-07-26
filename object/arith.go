package object

import "errors"

// RightAdder, RightSubtractor, RightMultiplier and RightDivider are the
// reflected halves of the arithmetic operators: they let a value complete an
// operation whose left operand does not recognize it, which is how `1 + money`
// reaches a user-defined __radd on money. Unlike comparison, arithmetic cannot
// simply swap its operands — `a - b` is not `b - a` — so the right operand
// needs a handler of its own.
//
// The argument is the LEFT operand of the expression: b.RMinus(a) computes
// `a - b`. The bool reports whether the receiver handled this operand at all;
// when it is false the left operand's original error stands, so a value can
// accept some types and stay silent about the rest.
type RightAdder interface {
	RAdd(left Object) (Object, bool, error)
}

type RightSubtractor interface {
	RMinus(left Object) (Object, bool, error)
}

type RightMultiplier interface {
	RMultiply(left Object) (Object, bool, error)
}

type RightDivider interface {
	RDivide(left Object) (Object, bool, error)
}

// Add, Minus, Multiply and Divide are the entry points both backends use for
// the arithmetic operators. They run the left operand's own method first and
// fall back to the right operand's reflected method when the left one reports
// that it does not know the type — the same rule Compare and Equals follow, so
// only a TypeError triggers the fallback and every other failure propagates.
//
// Numeric operands, which every arithmetic-heavy loop is made of, are answered
// straight from the shared numeric helpers: no reflected dispatch can apply to
// them, so the general path's interface call would be pure overhead.
func Add(a, b Object) (Object, error) {
	if result, ok := numericAdd(a, b); ok {
		return result, nil
	}
	res, err := a.Add(b)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, TypeError) {
		return nil, err
	}
	if right, ok := b.(RightAdder); ok {
		if res, handled, rerr := right.RAdd(a); handled {
			return res, rerr
		}
	}
	return nil, err
}

func Minus(a, b Object) (Object, error) {
	if result, ok := numericMinus(a, b); ok {
		return result, nil
	}
	res, err := a.Minus(b)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, TypeError) {
		return nil, err
	}
	if right, ok := b.(RightSubtractor); ok {
		if res, handled, rerr := right.RMinus(a); handled {
			return res, rerr
		}
	}
	return nil, err
}

func Multiply(a, b Object) (Object, error) {
	if result, ok := numericMultiply(a, b); ok {
		return result, nil
	}
	res, err := a.Multiply(b)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, TypeError) {
		return nil, err
	}
	if right, ok := b.(RightMultiplier); ok {
		if res, handled, rerr := right.RMultiply(a); handled {
			return res, rerr
		}
	}
	return nil, err
}

func Divide(a, b Object) (Object, error) {
	if result, ok, err := numericDivide(a, b); ok {
		return result, err
	}
	res, err := a.Divide(b)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, TypeError) {
		return nil, err
	}
	if right, ok := b.(RightDivider); ok {
		if res, handled, rerr := right.RDivide(a); handled {
			return res, rerr
		}
	}
	return nil, err
}
