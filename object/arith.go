package object

import "errors"

// Add, Minus, Multiply, Divide and Modulo are the entry points both backends
// use for the arithmetic operators. They run the left operand's own method
// first and fall back to the right operand's reflected method (RAdd and
// friends, the Go side of __radd) when the left one reports that it does not
// know the type —
// the same rule Compare and Equals follow, so only a TypeError triggers the
// fallback and every other failure propagates. Unlike comparison, arithmetic
// cannot simply swap its operands — `a - b` is not `b - a` — which is why the
// right operand needs a handler of its own rather than a flipped result.
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
	if res, handled, rerr := b.RAdd(a); handled {
		return res, rerr
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
	if res, handled, rerr := b.RMinus(a); handled {
		return res, rerr
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
	if res, handled, rerr := b.RMultiply(a); handled {
		return res, rerr
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
	if res, handled, rerr := b.RDivide(a); handled {
		return res, rerr
	}
	return nil, err
}

func Modulo(a, b Object) (Object, error) {
	if result, ok, err := numericModulo(a, b); ok {
		return result, err
	}
	res, err := a.Modulo(b)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, TypeError) {
		return nil, err
	}
	if res, handled, rerr := b.RModulo(a); handled {
		return res, rerr
	}
	return nil, err
}
