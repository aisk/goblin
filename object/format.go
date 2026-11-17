package object

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FormatSpec is a parsed format specifier, the text after `:` in an
// interpolation (`"{price:>8.2f}"`) or the second argument of format():
//
//	[[fill] align] [sign] [zero] [width] [group] [precision] [type]
//
// ParseFormatSpec("") is the spec that formats a value as print shows it;
// note that it has Precision -1, not the zero value's 0.
type FormatSpec struct {
	Fill      rune // 0 = none; only set together with Align
	Align     byte // 0, '<', '>', '^'
	Plus      bool
	Zero      bool
	Width     int
	Comma     bool
	Precision int  // -1 = none
	Type      byte // 0, 'd', 'x', 'X', 'b', 'o', 'f', 'e'
}

const (
	maxFormatWidth     = 1000
	maxFormatPrecision = 100
)

// FormatSpecError is a malformed specifier. Offset is the byte offset of the
// offending character in the specifier text, so a string literal can report
// the error at that character.
type FormatSpecError struct {
	Offset  int
	Message string
}

func (e *FormatSpecError) Error() string { return e.Message }

// ParseFormatSpec validates and parses specifier text. Every structural
// mistake is reported here; only whether a value's type accepts the
// specifier is left to Apply. The empty text is the zero spec.
func ParseFormatSpec(text string) (FormatSpec, *FormatSpecError) {
	spec := FormatSpec{Precision: -1}
	fail := func(offset int, reason string) (FormatSpec, *FormatSpecError) {
		return FormatSpec{}, &FormatSpecError{Offset: offset, Message: "invalid format specifier " + strconv.Quote(text) + ": " + reason}
	}
	if i := strings.IndexAny(text, "{}"); i >= 0 {
		msg := "invalid format specifier: a specifier cannot contain '" + text[i:i+1] + "'"
		if text[i] == '{' {
			msg += "; use format(value, spec)"
		}
		return FormatSpec{}, &FormatSpecError{Offset: i, Message: msg}
	}

	i := 0
	// The first character is a fill exactly when the second is an align.
	first, size := utf8.DecodeRuneInString(text)
	if len(text) > size && isFormatAlign(text[size]) {
		spec.Fill, spec.Align = first, text[size]
		i = size + 1
	} else if len(text) > 0 && isFormatAlign(text[0]) {
		spec.Align = text[0]
		i = 1
	} else if len(text) > size && !strings.ContainsRune("+0123456789,.dxXbofe", first) {
		return fail(0, "a fill character needs '<', '>' or '^' after it")
	}

	if i < len(text) && text[i] == '+' {
		spec.Plus = true
		i++
	}
	if i < len(text) && text[i] == '0' {
		if spec.Align != 0 {
			return fail(i, "'0' cannot be combined with an alignment")
		}
		spec.Zero = true
		i++
	}
	if start := i; i < len(text) && text[i] >= '1' && text[i] <= '9' {
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			i++
		}
		width, err := strconv.Atoi(text[start:i])
		if err != nil || width > maxFormatWidth {
			return fail(start, "width must be at most 1000")
		}
		spec.Width = width
	}
	comma := -1
	if i < len(text) && text[i] == ',' {
		spec.Comma = true
		comma = i
		i++
	}
	dot := -1
	if i < len(text) && text[i] == '.' {
		dot = i
		i++
		start := i
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			i++
		}
		if start == i {
			return fail(dot, "'.' needs the number of digits after it")
		}
		precision, err := strconv.Atoi(text[start:i])
		if err != nil || precision > maxFormatPrecision {
			return fail(start, "precision must be at most 100")
		}
		spec.Precision = precision
	}
	if i < len(text) && strings.IndexByte("dxXbofe", text[i]) >= 0 {
		spec.Type = text[i]
		i++
	}
	if i < len(text) {
		r, _ := utf8.DecodeRuneInString(text[i:])
		return fail(i, "unexpected '"+string(r)+"'")
	}

	switch spec.Type {
	case 'f', 'e':
		if spec.Comma && spec.Type == 'e' {
			return fail(comma, "',' cannot be used with 'e'")
		}
	case 0:
		if spec.Precision >= 0 {
			return fail(dot, "precision needs 'f' or 'e'")
		}
	case 'd':
		if spec.Precision >= 0 {
			return fail(dot, "precision cannot be used with 'd'")
		}
	default: // 'x', 'X', 'b', 'o'
		if spec.Precision >= 0 {
			return fail(dot, "precision cannot be used with '"+string(spec.Type)+"'")
		}
		if spec.Comma {
			return fail(comma, "',' cannot be used with '"+string(spec.Type)+"'")
		}
	}
	return spec, nil
}

func isFormatAlign(c byte) bool { return c == '<' || c == '>' || c == '^' }

// String renders the spec as it is written. Each spec has one spelling, so
// this is the text it was parsed from.
func (s FormatSpec) String() string {
	var b strings.Builder
	if s.Fill != 0 {
		b.WriteRune(s.Fill)
	}
	if s.Align != 0 {
		b.WriteByte(s.Align)
	}
	if s.Plus {
		b.WriteByte('+')
	}
	if s.Zero {
		b.WriteByte('0')
	}
	if s.Width > 0 {
		b.WriteString(strconv.Itoa(s.Width))
	}
	if s.Comma {
		b.WriteByte(',')
	}
	if s.Precision >= 0 {
		b.WriteByte('.')
		b.WriteString(strconv.Itoa(s.Precision))
	}
	if s.Type != 0 {
		b.WriteByte(s.Type)
	}
	return b.String()
}

// numeric reports whether the spec uses a part only numbers accept.
func (s FormatSpec) numeric() bool {
	return s.Plus || s.Zero || s.Comma || s.Precision >= 0 || s.Type != 0
}

// Apply renders v under the spec. Numbers take every part; any other value
// takes only fill, align and width, applied to the text print shows. The
// errors are a value whose type does not accept the spec, and v's own
// ToString error.
func (s FormatSpec) Apply(v Object) (Object, error) {
	switch n := v.(type) {
	case Integer:
		switch s.Type {
		case 'f', 'e':
			return s.formatFloat(float64(n)), nil
		}
		return s.formatInt(int64(n)), nil
	case Float:
		switch s.Type {
		case 'd', 'x', 'X', 'b', 'o':
			return nil, NewTypeError("format specifier %q needs an Int, got Float", s.String())
		case 0:
			if s.Comma {
				return nil, NewTypeError("format specifier %q needs an Int, got Float", s.String())
			}
		}
		return s.formatFloat(float64(n)), nil
	}
	if s.numeric() {
		return nil, NewTypeError("format specifier %q needs an Int or Float, got %s", s.String(), v.TypeName())
	}
	text, err := v.ToString()
	if err != nil {
		return nil, err
	}
	return String(s.pad(text, '<')), nil
}

func (s FormatSpec) formatInt(v int64) String {
	base := 10
	switch s.Type {
	case 'x', 'X':
		base = 16
	case 'b':
		base = 2
	case 'o':
		base = 8
	}
	digits := strconv.FormatUint(absInt64(v), base)
	if s.Type == 'X' {
		digits = strings.ToUpper(digits)
	}
	if s.Comma {
		digits = groupThousands(digits)
	}
	return String(s.number(v < 0, digits))
}

// absInt64 is |v| as an unsigned value, which also covers math.MinInt64.
func absInt64(v int64) uint64 {
	if v < 0 {
		return uint64(-(v + 1)) + 1
	}
	return uint64(v)
}

func (s FormatSpec) formatFloat(v float64) String {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		// Shown as print shows them; zero padding becomes spaces.
		text := Float(v).String()
		return String(s.pad(text, '>'))
	}
	var text string
	switch s.Type {
	case 'f', 'e':
		precision := s.Precision
		if precision < 0 {
			precision = 6
		}
		text = strconv.FormatFloat(v, s.Type, precision, 64)
	default:
		text = Float(v).String()
	}
	negative := strings.HasPrefix(text, "-")
	digits := strings.TrimPrefix(text, "-")
	if s.Comma {
		intPart, rest := digits, ""
		if dot := strings.IndexByte(digits, '.'); dot >= 0 {
			intPart, rest = digits[:dot], digits[dot:]
		}
		digits = groupThousands(intPart) + rest
	}
	return String(s.number(negative, digits))
}

// number puts the sign in front of digits and pads the result: with zeros
// between the sign and the digits under '0', otherwise with the fill.
func (s FormatSpec) number(negative bool, digits string) string {
	sign := ""
	if negative {
		sign = "-"
	} else if s.Plus {
		sign = "+"
	}
	if s.Zero {
		if n := s.Width - len(sign) - utf8.RuneCountInString(digits); n > 0 {
			return sign + strings.Repeat("0", n) + digits
		}
		return sign + digits
	}
	return s.pad(sign+digits, '>')
}

// pad widens text to the spec's width with its fill character, aligned as
// the spec says or by def. Centred text puts the odd space on the right.
func (s FormatSpec) pad(text string, def byte) string {
	n := s.Width - utf8.RuneCountInString(text)
	if n <= 0 {
		return text
	}
	fill := " "
	if s.Fill != 0 {
		fill = string(s.Fill)
	}
	align := s.Align
	if align == 0 {
		align = def
	}
	switch align {
	case '<':
		return text + strings.Repeat(fill, n)
	case '^':
		left := n / 2
		return strings.Repeat(fill, left) + text + strings.Repeat(fill, n-left)
	}
	return strings.Repeat(fill, n) + text
}

// groupThousands inserts a ',' between every three decimal digits.
func groupThousands(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	head := len(digits) % 3
	if head > 0 {
		b.WriteString(digits[:head])
	}
	for i := head; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// Format is the global format(value, spec): `"{x:spec}"` written with a spec
// known only at run time. An invalid spec is a ValueError.
func Format(v Object, spec string) (Object, error) {
	s, err := ParseFormatSpec(spec)
	if err != nil {
		return nil, NewValueError("format() %s", err.Message)
	}
	return s.Apply(v)
}
