package object

// Every position a built-in sequence takes is one of two kinds, and these are
// the only two functions that resolve them, so List, String and Bytes count
// from the end and report a bad position the same way.
//
//   - An element index names an element: -size <= i < size.
//   - A bound names a gap between elements, the ends included:
//     -size <= i <= size. Slice ends, insert positions and the start of an
//     index() search are bounds.
//
// A negative position counts from the end. Anything outside the range raises
// IndexError rather than being clamped.

// ResolveIndex resolves the element index i of a sequence of size elements to
// an offset in [0, size). typeName names the sequence in the error, which
// shows i as the caller wrote it.
func ResolveIndex(typeName string, i Integer, size int) (int, error) {
	if n, ok := resolvePosition(i, size, size-1); ok {
		return n, nil
	}
	return 0, NewIndexError("%s index out of range: %d", typeName, int64(i))
}

// ResolveBound resolves the bound i of a sequence of size elements to an
// offset in [0, size].
func ResolveBound(typeName string, i Integer, size int) (int, error) {
	if n, ok := resolvePosition(i, size, size); ok {
		return n, nil
	}
	return 0, NewIndexError("%s index out of range: %d", typeName, int64(i))
}

// resolvePosition counts a negative i from the end and reports whether the
// result lies in [0, max].
func resolvePosition(i Integer, size, max int) (int, bool) {
	n := int64(i)
	if n < 0 {
		n += int64(size)
	}
	if n < 0 || n > int64(max) {
		return 0, false
	}
	return int(n), true
}

// indexArg checks that index, the operand of `x[index]` on a sequence, is an
// Int.
func indexArg(typeName string, index Object) (Integer, error) {
	i, ok := index.(Integer)
	if !ok {
		return 0, NewTypeError("%s index must be Int, got %s", typeName, index.TypeName())
	}
	return i, nil
}
