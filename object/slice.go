package object

import "unicode/utf8"

// Slice performs `obj[low:high]`. A nil bound was omitted in the source: low
// then defaults to zero and high to the size. Lists, strings and Bytes can be
// sliced; the result is always a new value that shares nothing with obj.
//
// Both ends are Ints; a negative one counts from the end. A slice takes the
// elements that exist between its ends: an end beyond either end of the
// sequence is moved to that end (see ClampBound), and a low end past the
// high one gives an empty result. `xs[:3]` on a two-element list is the two
// elements, not an error.
func Slice(obj, low, high Object) (Object, error) {
	switch v := obj.(type) {
	case *List:
		lo, hi, err := sliceBounds(v, low, high, len(v.Elements))
		if err != nil {
			return nil, err
		}
		return &List{Elements: append([]Object(nil), v.Elements[lo:hi]...)}, nil
	case Bytes:
		lo, hi, err := sliceBounds(v, low, high, len(v))
		if err != nil {
			return nil, err
		}
		return NewBytes(v[lo:hi]), nil
	case String:
		// Positions count characters, as size and iteration do.
		s := string(v)
		lo, hi, err := sliceBounds(v, low, high, utf8.RuneCountInString(s))
		if err != nil {
			return nil, err
		}
		start := runeOffset(s, lo)
		return String(s[start : start+runeOffset(s[start:], hi-lo)]), nil
	}
	return nil, NewTypeError("%s cannot be sliced", obj.TypeName())
}

// sliceBounds resolves the bounds of a slice of obj, whose size is given, to
// offsets with 0 <= lo <= hi <= size.
func sliceBounds(obj Object, low, high Object, size int) (lo, hi int, err error) {
	lo, hi = 0, size
	if low != nil {
		i, err := sliceBound(obj, low)
		if err != nil {
			return 0, 0, err
		}
		lo = ClampBound(i, size)
	}
	if high != nil {
		i, err := sliceBound(obj, high)
		if err != nil {
			return 0, 0, err
		}
		hi = ClampBound(i, size)
	}
	if lo > hi {
		lo = hi
	}
	return lo, hi, nil
}

func sliceBound(obj, bound Object) (Integer, error) {
	i, ok := bound.(Integer)
	if !ok {
		return 0, NewTypeError("%s slice bound must be Int, got %s", obj.TypeName(), bound.TypeName())
	}
	return i, nil
}

// runeOffset returns the byte offset of the n-th character of s, or len(s)
// when s has exactly n characters.
func runeOffset(s string, n int) int {
	offset := 0
	for ; n > 0; n-- {
		_, width := utf8.DecodeRuneInString(s[offset:])
		offset += width
	}
	return offset
}

// Unpack iterates value for a destructuring statement with count targets
// and returns its elements, which must number exactly count. A List's
// elements are copied: the stores run Goblin code (an index or attribute
// expression) that may change the list before the last element is read.
func Unpack(value Object, count int) ([]Object, error) {
	items, err := Collect(value)
	if err != nil {
		return nil, err
	}
	if len(items) != count {
		return nil, NewValueError("cannot unpack %d values into %d targets", len(items), count)
	}
	if _, ok := value.(*List); ok {
		items = append([]Object(nil), items...)
	}
	return items, nil
}
