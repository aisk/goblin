package object

import "math"

// Integer is a 64-bit signed integer and Goblin has no big integers, so
// arithmetic whose result does not fit raises OverflowError rather than
// wrapping around. The checked operations below are shared by the runtime and
// by the code the transpiler emits for unboxed integer arithmetic, so both
// backends overflow at exactly the same points.

// ErrIntegerOverflow is the message of every integer overflow.
const ErrIntegerOverflow = "integer overflow"

// NewIntegerOverflow builds the error raised when integer arithmetic overflows.
// It is kept out of line so the overflow branch stays a cold call and does not
// bloat the arithmetic it guards.
//
//go:noinline
func NewIntegerOverflow() *Error { return NewOverflowError(ErrIntegerOverflow) }

// CheckedAdd returns a+b and whether it fit in an int64.
func CheckedAdd(a, b int64) (int64, bool) {
	c := a + b
	// Overflow flips the sign of c away from both operands.
	return c, (a^c)&(b^c) >= 0
}

// CheckedSub returns a-b and whether it fit in an int64.
func CheckedSub(a, b int64) (int64, bool) {
	c := a - b
	// Only operands of different signs can overflow, and then c takes b's sign.
	return c, (a^b)&(a^c) >= 0
}

// CheckedMul returns a*b and whether it fit in an int64.
func CheckedMul(a, b int64) (int64, bool) {
	// Operands that both fit in 32 bits cannot overflow, which covers almost
	// every multiplication and skips the division below.
	if uint64(a+math.MaxInt32+1) <= math.MaxUint32 && uint64(b+math.MaxInt32+1) <= math.MaxUint32 {
		return a * b, true
	}
	if a == 0 || b == 0 {
		return 0, true
	}
	c := a * b
	if c/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		return 0, false
	}
	return c, true
}

// CheckedDiv returns a/b, truncated, for a non-zero b, and whether it fit in an
// int64: only math.MinInt64 / -1 does not.
func CheckedDiv(a, b int64) (int64, bool) {
	if b == -1 && a == math.MinInt64 {
		return 0, false
	}
	return a / b, true
}

// CheckedNeg returns -a and whether it fit in an int64.
func CheckedNeg(a int64) (int64, bool) {
	return -a, a != math.MinInt64
}

// FloatValue returns v unchanged. Generated code spells float literals
// through it so that Go does not treat them as constants: Go evaluates
// constant expressions exactly, which would make 0.1 + 0.2 equal 0.3 in a
// compiled program while the interpreter, rounding every operation, says it
// does not.
func FloatValue(v float64) float64 { return v }

// checkRepeat reports OverflowError when repeating a sequence of size
// elements count times would not fit in an int, which Go's repeat helpers
// and make would otherwise turn into a panic.
func checkRepeat(size int, count Integer) error {
	if _, ok := CheckedMul(int64(size), int64(count)); !ok {
		return NewIntegerOverflow()
	}
	return nil
}
