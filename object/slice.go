package object

import "unicode/utf8"

// Slice performs `obj[low:high]`. A nil bound was omitted in the source: low
// then defaults to zero and high to the size. Lists, strings and Bytes can be
// sliced; the result is always a new value that shares nothing with obj.
//
// Both ends are bounds (see ResolveBound): Ints within -size <= i <= size,
// a negative one counting from the end. A bound outside that range, or a low
// end past the high one once both are resolved, raises IndexError rather
// than being clamped.
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

// sliceBounds resolves the bounds of a slice of obj, whose size is given. The
// error shows the bounds as written, an omitted one as its default.
func sliceBounds(obj Object, low, high Object, size int) (lo, hi int, err error) {
	rawLo, rawHi := Integer(0), Integer(size)
	if low != nil {
		if rawLo, err = sliceBound(obj, low); err != nil {
			return 0, 0, err
		}
	}
	if high != nil {
		if rawHi, err = sliceBound(obj, high); err != nil {
			return 0, 0, err
		}
	}
	lo, okLo := resolvePosition(rawLo, size, size)
	hi, okHi := resolvePosition(rawHi, size, size)
	if !okLo || !okHi || lo > hi {
		return 0, 0, NewIndexError("%s slice bounds out of range: [%d:%d] with size %d", obj.TypeName(), int64(rawLo), int64(rawHi), size)
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
// and returns its elements, which must number exactly count.
func Unpack(value Object, count int) ([]Object, error) {
	items, err := Collect(value)
	if err != nil {
		return nil, err
	}
	if len(items) != count {
		return nil, NewValueError("cannot unpack %d values into %d targets", len(items), count)
	}
	return items, nil
}
