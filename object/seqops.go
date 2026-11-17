package object

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The subsequence operations String and Bytes share: contains, count, index,
// replace, split and cut. Each is written once over the receiver's content as
// a Go string; seq says how the two types differ, in how a position is
// counted (characters for a String, bytes for Bytes) and in how a piece of
// the result is wrapped.
type seq struct {
	text     string
	typeName string
	chars    bool
}

func stringSeq(s String) seq { return seq{text: string(s), typeName: "String", chars: true} }
func bytesSeq(b Bytes) seq   { return seq{text: string(b), typeName: "Bytes"} }

// size is the number of positions: characters or bytes.
func (q seq) size() int {
	if q.chars {
		return utf8.RuneCountInString(q.text)
	}
	return len(q.text)
}

// offset converts a position to a byte offset in text.
func (q seq) offset(pos int) int {
	if q.chars {
		return runeOffset(q.text, pos)
	}
	return pos
}

// position counts the positions in part, a piece of text.
func (q seq) position(part string) int {
	if q.chars {
		return utf8.RuneCountInString(part)
	}
	return len(part)
}

func (q seq) wrap(s string) Object {
	if q.chars {
		return String(s)
	}
	return Bytes(s)
}

func (q seq) wrapAll(parts []string) *List {
	elements := make([]Object, len(parts))
	for i, part := range parts {
		elements[i] = q.wrap(part)
	}
	return &List{Elements: elements}
}

func (q seq) contains(value string) Object { return Bool(strings.Contains(q.text, value)) }

// count counts non-overlapping occurrences. An empty value occurs at every
// character boundary, as in Go: one more time than there are characters.
func (q seq) count(value string) Object { return Integer(strings.Count(q.text, value)) }

// index returns the position of the first occurrence of value at or after
// the bound start, or nil.
func (q seq) index(value string, start Integer) (Object, error) {
	from, err := ResolveBound(q.typeName, start, q.size())
	if err != nil {
		return nil, err
	}
	byteFrom := q.offset(from)
	i := strings.Index(q.text[byteFrom:], value)
	if i < 0 {
		return Nil, nil
	}
	return Integer(from + q.position(q.text[byteFrom:byteFrom+i])), nil
}

// replace replaces the first count occurrences of old, every one when count
// is absent. An empty old is refused, as split refuses an empty separator:
// it would match between every two characters.
func (q seq) replace(old, new string, count Integer, limited bool) (Object, error) {
	if old == "" {
		return nil, NewValueError("replace() argument 'old' must not be empty")
	}
	n := -1
	if limited {
		if count < 0 {
			return nil, NewValueError("replace() count must not be negative")
		}
		n = int(count)
	}
	return q.wrap(strings.Replace(q.text, old, new, n)), nil
}

// split cuts text at each sep, at most count times when count is given, so
// the result has at most count+1 pieces. Without a separator (hasSep false)
// it splits at runs of whitespace and drops empty pieces; a limited split
// then keeps the rest of the text, leading whitespace removed, as the last
// piece.
func (q seq) split(sep string, hasSep bool, count Integer, limited bool) (Object, error) {
	if hasSep && sep == "" {
		return nil, NewValueError("split() separator must not be empty")
	}
	n := -1
	if limited {
		if count < 0 {
			return nil, NewValueError("split() count must not be negative")
		}
		n = int(count)
	}
	if hasSep {
		if n >= 0 {
			n++
		}
		return q.wrapAll(strings.SplitN(q.text, sep, n)), nil
	}
	return q.wrapAll(splitSpace(q.text, n)), nil
}

// splitSpace splits s at runs of Unicode whitespace, at most n times when n
// is not negative.
func splitSpace(s string, n int) []string {
	parts := []string{}
	for {
		s = strings.TrimLeftFunc(s, unicode.IsSpace)
		if s == "" {
			return parts
		}
		if len(parts) == n {
			return append(parts, s)
		}
		end := strings.IndexFunc(s, unicode.IsSpace)
		if end < 0 {
			return append(parts, s)
		}
		parts = append(parts, s[:end])
		s = s[end:]
	}
}

// cut splits text around the first sep: [before, after], or [text, nil] when
// sep does not occur.
func (q seq) cut(sep string) Object {
	before, after, found := strings.Cut(q.text, sep)
	if !found {
		return &List{Elements: []Object{q.wrap(q.text), Nil}}
	}
	return &List{Elements: []Object{q.wrap(before), q.wrap(after)}}
}
