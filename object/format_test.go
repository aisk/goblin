package object

import (
	"math"
	"testing"
)

func TestFormatSpecApply(t *testing.T) {
	cases := []struct {
		value Object
		spec  string
		want  string
	}{
		// Width, align and fill on any value; numbers align right.
		{String("tea"), "", "tea"},
		{String("tea"), "8", "tea     "},
		{String("tea"), ">8", "     tea"},
		{String("tea"), "^8", "  tea   "},
		{String("tea"), "*^9", "***tea***"},
		{String("tea"), " >5", "  tea"},
		{String("héllo"), ".<7", "héllo.."},
		{String("longer"), "3", "longer"},
		{Integer(42), "5", "   42"},
		{Integer(42), "<5", "42   "},
		{Float(2), "6", "   2.0"},
		{True, ">6", "  true"},
		{Nil, "^5", " nil "},
		{&List{Elements: []Object{Integer(1)}}, "<5", "[1]  "},
		// Ints.
		{Integer(255), "d", "255"},
		{Integer(255), "x", "ff"},
		{Integer(255), "X", "FF"},
		{Integer(255), "b", "11111111"},
		{Integer(255), "o", "377"},
		{Integer(255), "08b", "11111111"},
		{Integer(5), "08b", "00000101"},
		{Integer(-255), "x", "-ff"},
		{Integer(-255), "06x", "-000ff"},
		{Integer(7), "03", "007"},
		{Integer(-42), "08", "-0000042"},
		{Integer(42), "+", "+42"},
		{Integer(0), "+", "+0"},
		{Integer(42), "+08", "+0000042"},
		{Integer(1234567), ",", "1,234,567"},
		{Integer(-1234567), ",d", "-1,234,567"},
		{Integer(123), ",", "123"},
		{Integer(1234), ">8,", "   1,234"},
		{Integer(math.MaxInt64), ",", "9,223,372,036,854,775,807"},
		{Integer(math.MinInt64), "x", "-8000000000000000"},
		{Integer(math.MinInt64), "", "-9223372036854775808"},
		{Integer(255), "f", "255.000000"},
		{Integer(255), ".1f", "255.0"},
		{Integer(1500), ".2e", "1.50e+03"},
		// Floats.
		{Float(3.14159), "f", "3.141590"},
		{Float(3.14159), ".2f", "3.14"},
		{Float(3.14159), ".0f", "3"},
		{Float(2.675), ".2f", "2.67"},
		{Float(-0.001), ".2f", "-0.00"},
		{Float(1500), "e", "1.500000e+03"},
		{Float(0.00012), ".3e", "1.200e-04"},
		{Float(0.5), "+.1f", "+0.5"},
		{Float(2), "+", "+2.0"},
		{Float(2), "06", "0002.0"},
		{Float(1e16), ">8", "   1e+16"},
		{Float(1234567.891), ",.2f", "1,234,567.89"},
		{Float(-1234.5), "+,.1f", "-1,234.5"},
		{Float(-7.5), "07.2f", "-007.50"},
		{Float(3.1), ">8.2f", "    3.10"},
		{Float(math.Inf(1)), "08.2f", "    +Inf"},
		{Float(math.Inf(-1)), "+e", "-Inf"},
		{Float(math.NaN()), "+.1f", "NaN"},
		{Float(math.NaN()), "<5", "NaN  "},
	}
	for _, c := range cases {
		got, err := Format(c.value, c.spec)
		if err != nil {
			t.Errorf("format(%v, %q): %v", c.value, c.spec, err)
			continue
		}
		if got != String(c.want) {
			t.Errorf("format(%v, %q) = %q, want %q", c.value, c.spec, got, c.want)
		}
	}
}

func TestParseFormatSpecErrors(t *testing.T) {
	cases := []struct {
		spec   string
		offset int
		want   string
	}{
		{".2", 0, `invalid format specifier ".2": precision needs 'f' or 'e'`},
		{"5.2x", 1, `invalid format specifier "5.2x": precision cannot be used with 'x'`},
		{"5.2d", 1, `invalid format specifier "5.2d": precision cannot be used with 'd'`},
		{",x", 0, `invalid format specifier ",x": ',' cannot be used with 'x'`},
		{",e", 0, `invalid format specifier ",e": ',' cannot be used with 'e'`},
		{"<05", 1, `invalid format specifier "<05": '0' cannot be combined with an alignment`},
		{"*5", 0, `invalid format specifier "*5": a fill character needs '<', '>' or '^' after it`},
		{"5q", 1, `invalid format specifier "5q": unexpected 'q'`},
		{"2f.1", 2, `invalid format specifier "2f.1": unexpected '.'`},
		{"q", 0, `invalid format specifier "q": unexpected 'q'`},
		{"2000", 0, `invalid format specifier "2000": width must be at most 1000`},
		{".200f", 1, `invalid format specifier ".200f": precision must be at most 100`},
		{".f", 0, `invalid format specifier ".f": '.' needs the number of digits after it`},
		{"{w}", 0, `invalid format specifier: a specifier cannot contain '{'; use format(value, spec)`},
		{">5}", 2, `invalid format specifier: a specifier cannot contain '}'`},
	}
	for _, c := range cases {
		_, err := ParseFormatSpec(c.spec)
		if err == nil {
			t.Errorf("%q: no error", c.spec)
			continue
		}
		if err.Message != c.want || err.Offset != c.offset {
			t.Errorf("%q: error %q at %d, want %q at %d", c.spec, err.Message, err.Offset, c.want, c.offset)
		}
		if _, ferr := Format(Integer(1), c.spec); ferr == nil || ferr.Error() != "format() "+c.want {
			t.Errorf("format(1, %q): %v", c.spec, ferr)
		}
	}
	// Width and precision at their limits are accepted.
	for _, spec := range []string{"1000", ".100f", "0", "+", ","} {
		if _, err := ParseFormatSpec(spec); err != nil {
			t.Errorf("%q: %v", spec, err)
		}
	}
}

func TestFormatSpecTypeErrors(t *testing.T) {
	cases := []struct {
		value Object
		spec  string
		want  string
	}{
		{String("tea"), ".2f", `format specifier ".2f" needs an Int or Float, got String`},
		{True, "d", `format specifier "d" needs an Int or Float, got Bool`},
		{Nil, "+", `format specifier "+" needs an Int or Float, got Nil`},
		{Float(2.5), "x", `format specifier "x" needs an Int, got Float`},
		{Float(2.5), "d", `format specifier "d" needs an Int, got Float`},
		{Float(2.5), ",", `format specifier "," needs an Int, got Float`},
		{Float(2.5), ">10,", `format specifier ">10," needs an Int, got Float`},
	}
	for _, c := range cases {
		_, err := Format(c.value, c.spec)
		if err == nil || err.Error() != c.want {
			t.Errorf("format(%v, %q): %v, want %q", c.value, c.spec, err, c.want)
			continue
		}
		if ok, _ := ErrorMatches(ErrorValue(err), TypeError); !ok {
			t.Errorf("format(%v, %q): not a TypeError", c.value, c.spec)
		}
	}
}

// String re-renders a spec as written, which the run-time errors rely on.
func TestFormatSpecString(t *testing.T) {
	for _, spec := range []string{"", "<8", "*^9", " >5", "→>3", "+08", ",", "+,.2f", ">10,", "08b", ".3e", "X", "0", "1000"} {
		s, err := ParseFormatSpec(spec)
		if err != nil {
			t.Fatalf("%q: %v", spec, err)
		}
		if got := s.String(); got != spec {
			t.Errorf("ParseFormatSpec(%q).String() = %q", spec, got)
		}
	}
}
