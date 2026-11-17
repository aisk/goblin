package object

// Every position a built-in sequence takes names either an element or a
// boundary between elements, and the functions here are the only ones that
// resolve them, so List, String and Bytes count from the end and report a
// bad position the same way.
//
//   - An element index names an element: -size <= i < size. Indexing
//     (`x[i]`, `x[i] = v`) and pop(index) take one.
//   - A bound names a gap between elements, the ends included:
//     -size <= i <= size. Insert positions and the start of an index()
//     search are bounds.
//
// A negative position counts from the end. An element index or a bound
// outside its range raises IndexError: it names something that is not
// there. The ends of a slice are the exception: a slice takes the elements
// that exist between its ends, so ClampBound moves an end beyond the
// sequence to the nearest end instead of raising.

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

// ClampBound resolves the slice end i of a sequence of size elements to an
// offset in [0, size]: a negative i counts from the end, and the result is
// moved to 0 or size when it falls outside.
func ClampBound(i Integer, size int) int {
	n := int64(i)
	if n < 0 {
		n += int64(size)
	}
	if n < 0 {
		return 0
	}
	if n > int64(size) {
		return size
	}
	return int(n)
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
