package timevalue

import (
	"strconv"
	"strings"
	stdtime "time"

	"github.com/aisk/goblin/object"
)

// Layouts are strftime directives, formatted and scanned here directly.
// They are deliberately not translated to Go reference layouts: Go has no
// escape for literal text, so "2006" or "Jan" written as text in a layout
// would be read as a directive.

var (
	monthNames   = [...]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	weekdayNames = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

func unknownDirective(fn string, d byte) error {
	if d == 0 {
		return object.NewValueError("%s() layout ends with a lone %%", fn)
	}
	return object.NewValueError("%s() unknown layout directive %%%c", fn, d)
}

func appendPadded(b []byte, n, width int, pad byte) []byte {
	s := strconv.Itoa(n)
	if n < 0 {
		b = append(b, '-')
		s = s[1:]
		width--
	}
	for i := len(s); i < width; i++ {
		b = append(b, pad)
	}
	return append(b, s...)
}

// Format writes t per layout; fn names the method in a ValueError for an
// unknown directive.
func Format(fn string, t stdtime.Time, layout string) (string, error) {
	b := make([]byte, 0, len(layout)+16)
	for i := 0; i < len(layout); i++ {
		c := layout[i]
		if c != '%' {
			b = append(b, c)
			continue
		}
		i++
		if i == len(layout) {
			return "", unknownDirective(fn, 0)
		}
		switch layout[i] {
		case 'Y':
			b = appendPadded(b, t.Year(), 4, '0')
		case 'y':
			b = appendPadded(b, (t.Year()%100+100)%100, 2, '0')
		case 'm':
			b = appendPadded(b, int(t.Month()), 2, '0')
		case 'd':
			b = appendPadded(b, t.Day(), 2, '0')
		case 'e':
			b = appendPadded(b, t.Day(), 2, ' ')
		case 'H':
			b = appendPadded(b, t.Hour(), 2, '0')
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			b = appendPadded(b, h, 2, '0')
		case 'M':
			b = appendPadded(b, t.Minute(), 2, '0')
		case 'S':
			b = appendPadded(b, t.Second(), 2, '0')
		case 'f':
			b = appendPadded(b, t.Nanosecond()/1000, 6, '0')
		case 'p':
			if t.Hour() < 12 {
				b = append(b, "AM"...)
			} else {
				b = append(b, "PM"...)
			}
		case 'A':
			b = append(b, weekdayNames[t.Weekday()]...)
		case 'a':
			b = append(b, weekdayNames[t.Weekday()][:3]...)
		case 'B':
			b = append(b, monthNames[t.Month()-1]...)
		case 'b':
			b = append(b, monthNames[t.Month()-1][:3]...)
		case 'j':
			b = appendPadded(b, t.YearDay(), 3, '0')
		case 'z':
			_, offset := t.Zone()
			b = append(b, offsetText(offset, false)...)
		case 'Z':
			name, offset := t.Zone()
			if name == "" {
				name = offsetText(offset, true)
			}
			b = append(b, name...)
		case 'G':
			year, _ := t.ISOWeek()
			b = appendPadded(b, year, 4, '0')
		case 'V':
			_, week := t.ISOWeek()
			b = appendPadded(b, week, 2, '0')
		case '%':
			b = append(b, '%')
		default:
			return "", unknownDirective(fn, layout[i])
		}
	}
	return string(b), nil
}

// wallClock is what a scan reads; fields the text does not give keep the
// defaults of C's strptime, 1900-01-01 00:00:00.
type wallClock struct {
	year, month, day     int
	hour, minute, second int
	nanos                int
	hour12, pm           bool
	offset               int
	hasOffset, utc       bool
}

func newWallClock() wallClock { return wallClock{year: 1900, month: 1, day: 1} }

// errNoMatch marks a scan that failed on the text, as opposed to a mistake
// in the layout; the caller turns it into its ParseError.
type errNoMatch struct{}

func (errNoMatch) Error() string { return "no match" }

// scanner reads text left to right.
type scanner struct {
	text string
	pos  int
}

// digits reads between min and max decimal digits.
func (s *scanner) digits(min, max int) (int, bool) {
	start := s.pos
	for s.pos < len(s.text) && s.pos-start < max && s.text[s.pos] >= '0' && s.text[s.pos] <= '9' {
		s.pos++
	}
	if s.pos-start < min {
		return 0, false
	}
	n, _ := strconv.Atoi(s.text[start:s.pos])
	return n, true
}

// fraction reads 1 to max digits as nanoseconds.
func (s *scanner) fraction(max int) (int, bool) {
	start := s.pos
	n, ok := s.digits(1, max)
	if !ok {
		return 0, false
	}
	for i := s.pos - start; i < 9; i++ {
		n *= 10
	}
	return n, true
}

// word reads one of names, case-insensitively, full or by its first three
// letters when short is set; it returns the index.
func (s *scanner) word(names []string, short bool) (int, bool) {
	rest := s.text[s.pos:]
	for i, name := range names {
		if short {
			name = name[:3]
		}
		if len(rest) >= len(name) && strings.EqualFold(rest[:len(name)], name) {
			s.pos += len(name)
			return i, true
		}
	}
	return 0, false
}

// offset reads "Z", "±HHMM" or "±HH:MM".
func (s *scanner) offset(w *wallClock) bool {
	if s.pos < len(s.text) && (s.text[s.pos] == 'Z' || s.text[s.pos] == 'z') {
		s.pos++
		w.hasOffset, w.utc = true, true
		return true
	}
	if s.pos >= len(s.text) || (s.text[s.pos] != '+' && s.text[s.pos] != '-') {
		return false
	}
	sign := 1
	if s.text[s.pos] == '-' {
		sign = -1
	}
	s.pos++
	hours, ok := s.digits(2, 2)
	if !ok {
		return false
	}
	if s.pos < len(s.text) && s.text[s.pos] == ':' {
		s.pos++
	}
	minutes, ok := s.digits(2, 2)
	if !ok || hours > 23 || minutes > 59 {
		return false
	}
	w.hasOffset = true
	w.offset = sign * (hours*3600 + minutes*60)
	return true
}

// scan reads text per a strftime layout. A layout mistake is a ValueError
// naming fn; text that does not fit is errNoMatch.
func scan(fn, text, layout string) (wallClock, error) {
	w := newWallClock()
	if err := checkLayout(fn, layout); err != nil {
		return w, err
	}
	s := &scanner{text: text}
	var ok bool
	for i := 0; i < len(layout); i++ {
		c := layout[i]
		if c != '%' {
			if s.pos >= len(text) || text[s.pos] != c {
				return w, errNoMatch{}
			}
			s.pos++
			continue
		}
		i++
		ok = true
		switch layout[i] {
		case 'Y':
			w.year, ok = s.digits(4, 4)
		case 'y':
			var y int
			y, ok = s.digits(2, 2)
			if y < 69 {
				w.year = 2000 + y
			} else {
				w.year = 1900 + y
			}
		case 'm':
			w.month, ok = s.digits(1, 2)
		case 'd':
			w.day, ok = s.digits(1, 2)
		case 'e':
			if s.pos < len(text) && text[s.pos] == ' ' {
				s.pos++
			}
			w.day, ok = s.digits(1, 2)
		case 'H':
			w.hour, ok = s.digits(1, 2)
		case 'I':
			w.hour, ok = s.digits(1, 2)
			w.hour12 = true
		case 'M':
			w.minute, ok = s.digits(1, 2)
		case 'S':
			w.second, ok = s.digits(1, 2)
		case 'f':
			w.nanos, ok = s.fraction(6)
		case 'p':
			var n int
			n, ok = s.word([]string{"AM", "PM"}, false)
			w.pm = n == 1
		case 'A':
			_, ok = s.word(weekdayNames[:], false)
		case 'a':
			_, ok = s.word(weekdayNames[:], true)
		case 'B':
			w.month, ok = s.word(monthNames[:], false)
			w.month++
		case 'b':
			w.month, ok = s.word(monthNames[:], true)
			w.month++
		case 'j':
			_, ok = s.digits(1, 3)
		case 'z':
			ok = s.offset(&w)
		case '%':
			ok = s.pos < len(text) && text[s.pos] == '%'
			s.pos++
		}
		if !ok {
			return w, errNoMatch{}
		}
	}
	if s.pos != len(text) {
		return w, errNoMatch{}
	}
	if w.hour12 {
		if w.hour < 1 || w.hour > 12 {
			return w, errNoMatch{}
		}
		w.hour %= 12
		if w.pm {
			w.hour += 12
		}
	}
	return w, nil
}

// checkLayout reports a directive scan would reject, without text.
func checkLayout(fn, layout string) error {
	for i := 0; i < len(layout); i++ {
		if layout[i] != '%' {
			continue
		}
		i++
		if i == len(layout) {
			return unknownDirective(fn, 0)
		}
		switch d := layout[i]; d {
		case 'Y', 'y', 'm', 'd', 'e', 'H', 'I', 'M', 'S', 'f', 'p', 'A', 'a', 'B', 'b', 'j', 'z', '%':
		case 'Z', 'G', 'V':
			return object.NewValueError("%s() layout directive %%%c can only be used to format", fn, d)
		default:
			return unknownDirective(fn, d)
		}
	}
	return nil
}

// build turns a scanned wall clock into an instant: in the offset the text
// gave, else in loc. Out-of-range fields do not roll over here: text that
// names the 31st of June does not name a day.
func (w wallClock) build(loc *stdtime.Location) (stdtime.Time, bool) {
	if w.month < 1 || w.month > 12 || w.day < 1 || w.hour > 23 || w.minute > 59 || w.second > 59 {
		return stdtime.Time{}, false
	}
	if w.day > daysIn(w.year, w.month) {
		return stdtime.Time{}, false
	}
	if w.hasOffset {
		if w.utc {
			loc = stdtime.UTC
		} else {
			loc = offsetZone(w.offset)
		}
	}
	return stdtime.Date(w.year, stdtime.Month(w.month), w.day, w.hour, w.minute, w.second, w.nanos, loc), true
}

func daysIn(year, month int) int {
	return stdtime.Date(year, stdtime.Month(month)+1, 0, 0, 0, 0, 0, stdtime.UTC).Day()
}

// parseISO reads the ISO 8601 / RFC 3339 forms parse() accepts with no
// layout: a date, optionally "T" or " " and hh:mm[:ss[.fraction]], then
// optionally "Z" or an offset.
func parseISO(text string) (wallClock, bool) {
	w := newWallClock()
	s := &scanner{text: text}
	lit := func(c byte) bool {
		if s.pos < len(text) && text[s.pos] == c {
			s.pos++
			return true
		}
		return false
	}
	var ok bool
	if w.year, ok = s.digits(4, 4); !ok || !lit('-') {
		return w, false
	}
	if w.month, ok = s.digits(2, 2); !ok || !lit('-') {
		return w, false
	}
	if w.day, ok = s.digits(2, 2); !ok {
		return w, false
	}
	if s.pos == len(text) {
		return w, true
	}
	if !lit('T') && !lit(' ') {
		return w, false
	}
	if w.hour, ok = s.digits(2, 2); !ok || !lit(':') {
		return w, false
	}
	if w.minute, ok = s.digits(2, 2); !ok {
		return w, false
	}
	if lit(':') {
		if w.second, ok = s.digits(2, 2); !ok {
			return w, false
		}
		if lit('.') {
			if w.nanos, ok = s.fraction(9); !ok {
				return w, false
			}
		}
	}
	if s.pos < len(text) && !s.offset(&w) {
		return w, false
	}
	return w, s.pos == len(text)
}

// Parse is time.parse: text per layout, or as ISO 8601 when layout is
// absent, in loc unless the text gives an offset. A mismatch is a
// ParseError carrying text and layout (nil when none was given).
func Parse(fn, text string, layout object.String, hasLayout bool, loc *stdtime.Location) (*Time, error) {
	var w wallClock
	ok := true
	if hasLayout {
		var err error
		w, err = scan(fn, text, string(layout))
		if err != nil {
			if _, mismatch := err.(errNoMatch); !mismatch {
				return nil, err
			}
			ok = false
		}
	} else {
		w, ok = parseISO(text)
	}
	var t stdtime.Time
	if ok {
		t, ok = w.build(loc)
	}
	if ok {
		return NewTime(t), nil
	}
	if hasLayout {
		return nil, object.NewParseError("%s() %s does not match layout %s", fn, strconv.Quote(text), strconv.Quote(string(layout))).
			With("text", object.String(text)).With("layout", layout)
	}
	return nil, object.NewParseError("%s() %s is not an ISO 8601 time", fn, strconv.Quote(text)).
		With("text", object.String(text)).With("layout", object.Nil)
}
