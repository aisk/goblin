package time

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	stdtime "time"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/timevalue"
	"github.com/aisk/goblin/object"
)

// Every test names its zones: none depends on the machine's local zone, so
// the package passes under any TZ.

func call(name string, args object.Args, kw ...object.KwArg) (object.Object, error) {
	m, _ := Execute()
	return m.(*object.Module).Members[name].(*object.Function).Call(object.CallArgs{Positional: args, Keyword: kw})
}

func kw(name string, v object.Object) object.KwArg { return object.KwArg{Name: name, Value: v} }

func str(s string) object.Object { return object.String(s) }

func mustCall(t *testing.T, name string, args object.Args, kws ...object.KwArg) object.Object {
	t.Helper()
	v, err := call(name, args, kws...)
	if err != nil {
		t.Fatalf("%s(%v) error = %v", name, args, err)
	}
	return v
}

func newTime(t *testing.T, args object.Args, kws ...object.KwArg) *timevalue.Time {
	t.Helper()
	v, err := timevalue.Type.Function.Call(object.CallArgs{Positional: args, Keyword: kws})
	if err != nil {
		t.Fatalf("Time(%v) error = %v", args, err)
	}
	return v.(*timevalue.Time)
}

func attr(t *testing.T, v object.Object, name string) object.Object {
	t.Helper()
	got, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func method(t *testing.T, v object.Object, name string, args ...object.Object) (object.Object, error) {
	t.Helper()
	return attr(t, v, name).(*object.Function).Call(object.CallArgs{Positional: args})
}

func mustMethod(t *testing.T, v object.Object, name string, args ...object.Object) object.Object {
	t.Helper()
	got, err := method(t, v, name, args...)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return got
}

func text(v object.Object) string { s, _ := v.ToString(); return s }

func ints(args ...int) object.Args {
	out := make(object.Args, len(args))
	for i, a := range args {
		out[i] = object.Integer(a)
	}
	return out
}

func TestConstants(t *testing.T) {
	m, _ := Execute()
	members := m.(*object.Module).Members
	for name, want := range map[string]object.Integer{"MINUTE": 60, "HOUR": 3600, "DAY": 86400} {
		if members[name] != want {
			t.Errorf("%s = %v", name, members[name])
		}
	}
	if members["Time"] != timevalue.Type.Function {
		t.Fatal("time.Time is not the Time constructor")
	}
}

func TestShowAndProperties(t *testing.T) {
	tm := newTime(t, ints(2026, 7, 19, 9, 30, 15), kw("zone", str("+08:00")))
	if got := tm.String(); got != "2026-07-19T09:30:15+08:00" {
		t.Fatalf("Show = %s", got)
	}
	list := &object.List{Elements: object.Args{tm}}
	if got, _ := list.ToString(); got != `[Time("2026-07-19T09:30:15+08:00")]` {
		t.Fatalf("Time in a List shows as %s", got)
	}
	want := map[string]object.Object{
		"year": object.Integer(2026), "month": object.Integer(7), "day": object.Integer(19),
		"hour": object.Integer(9), "minute": object.Integer(30), "second": object.Integer(15),
		"nanosecond": object.Integer(0), "weekday": object.Integer(7), "unix": object.Integer(1784424615),
		"zone": str("+08:00"), "offset": object.Integer(28800),
	}
	for name, v := range want {
		if got := attr(t, tm, name); !modtest.Equal(got, v) {
			t.Errorf("%s = %v, want %v", name, got, v)
		}
	}
	if truthy, _ := tm.ToBool(); !truthy {
		t.Fatal("a Time is always truthy")
	}
	frac := mustCall(t, "unix", object.Args{object.Float(-0.25)})
	if got := text(mustMethod(t, frac, "to_utc")); got != "1969-12-31T23:59:59.75Z" {
		t.Fatalf("unix(-0.25) = %s", got)
	}
	if got := attr(t, frac, "unix"); got != object.Integer(-1) {
		t.Fatalf("unix rounds down, got %v", got)
	}
}

func TestZones(t *testing.T) {
	cases := []struct{ zone, want, show string }{
		{"UTC", "UTC", "2026-01-15T12:00:00Z"},
		{"Asia/Shanghai", "Asia/Shanghai", "2026-01-15T12:00:00+08:00"},
		{"-03:30", "-03:30", "2026-01-15T12:00:00-03:30"},
		{"local", "local", ""},
	}
	for _, c := range cases {
		tm := newTime(t, ints(2026, 1, 15, 12), kw("zone", str(c.zone)))
		if got := attr(t, tm, "zone"); got != str(c.want) {
			t.Errorf("zone %q reports %v", c.zone, got)
		}
		if c.show != "" && tm.String() != c.show {
			t.Errorf("zone %q shows %s", c.zone, tm.String())
		}
		// The reported spelling resolves to the same zone.
		back := newTime(t, ints(2026, 1, 15, 12), kw("zone", attr(t, tm, "zone")))
		if !back.Value.Equal(tm.Value) {
			t.Errorf("zone %q does not round-trip", c.zone)
		}
	}
	if attr(t, newTime(t, ints(2026, 1, 15)), "zone") != str("local") {
		t.Fatal("zone=nil is not local")
	}
	// A name is matched exactly, also on a case-insensitive file system.
	for _, bad := range []string{"Asia/Shangai", "", "Local", "+8:00", "+24:00", "CST6CDT/x", "../etc", "utc", "asia/shanghai", "ASIA/Shanghai", "Asia/shanghai", "europe/london"} {
		_, err := timevalue.Type.Function.Call(object.CallArgs{Positional: ints(2026, 1, 1), Keyword: object.Kwargs{kw("zone", str(bad))}})
		if !errors.Is(err, object.ValueError) || !strings.Contains(err.Error(), `Time() unknown time zone "`+bad+`"`) {
			t.Errorf("Time(zone=%q) = %v", bad, err)
		}
	}
	_, err := method(t, newTime(t, ints(2026, 1, 1)), "to_zone", str("Mars/Base"))
	if err == nil || err.Error() != `to_zone() unknown time zone "Mars/Base"` {
		t.Fatalf("to_zone = %v", err)
	}
	_, err = call("parse", object.Args{str("2026-01-01")}, kw("zone", str("Nowhere")))
	if err == nil || err.Error() != `parse() unknown time zone "Nowhere"` {
		t.Fatalf("parse zone = %v", err)
	}
}

func TestToZoneKeepsInstant(t *testing.T) {
	meeting := newTime(t, ints(2026, 7, 19, 9, 30), kw("zone", str("America/New_York")))
	shanghai := mustMethod(t, meeting, "to_zone", str("Asia/Shanghai"))
	if text(shanghai) != "2026-07-19T21:30:00+08:00" || !modtest.Equal(shanghai, meeting) {
		t.Fatalf("to_zone = %s", text(shanghai))
	}
	paris := mustMethod(t, meeting, "to_zone", str("Europe/Paris"))
	if got := text(mustMethod(t, paris, "format", str("%H:%M %Z"))); got != "15:30 CEST" {
		t.Fatalf("Paris = %s", got)
	}
	utc := mustMethod(t, meeting, "to_utc")
	if text(utc) != "2026-07-19T13:30:00Z" || attr(t, utc, "zone") != str("UTC") {
		t.Fatalf("to_utc = %s", text(utc))
	}
	if !modtest.Equal(utc, mustMethod(t, meeting, "to_zone", str("UTC"))) {
		t.Fatal("to_utc() != to_zone(\"UTC\")")
	}
	if attr(t, meeting, "offset") != object.Integer(-4*3600) {
		t.Fatal("New York summer offset")
	}
}

// Daylight saving in New York, 2026: clocks go forward at 02:00 on March 8
// and back at 02:00 on November 1.
func TestDaylightSaving(t *testing.T) {
	ny := kw("zone", str("America/New_York"))
	before := newTime(t, ints(2026, 3, 8, 1, 59, 59), ny)
	after, _ := before.Add(object.Integer(1))
	if text(after) != "2026-03-08T03:00:00-04:00" || attr(t, after, "offset") != object.Integer(-4*3600) {
		t.Fatalf("one second after 01:59:59 EST is %s", text(after))
	}
	if got := text(mustMethod(t, before, "format", str("%Z"))); got != "EST" {
		t.Fatalf("%%Z before = %s", got)
	}
	if got := text(mustMethod(t, after, "format", str("%Z %z"))); got != "EDT -0400" {
		t.Fatalf("%%Z after = %s", got)
	}
	// The day of the change is 23 hours long, and start_of_day stays in it.
	noon := newTime(t, ints(2026, 3, 8, 12), ny)
	prev := newTime(t, ints(2026, 3, 7, 12), ny)
	if d, _ := noon.Minus(prev); d != object.Float(23*3600) {
		t.Fatalf("noon to noon across spring = %v", d)
	}
	start := mustMethod(t, noon, "start_of_day")
	if text(start) != "2026-03-08T00:00:00-05:00" {
		t.Fatalf("start_of_day = %s", text(start))
	}
	// t + DAY is 24 hours; the constructor steps the calendar.
	plusDay, _ := prev.Add(object.Integer(86400))
	if text(plusDay) != "2026-03-08T13:00:00-04:00" {
		t.Fatalf("t + DAY = %s", text(plusDay))
	}
	// The skipped hour gives one of the two possible instants.
	skipped := newTime(t, ints(2026, 3, 8, 2, 30), ny).String()
	if skipped != "2026-03-08T03:30:00-04:00" && skipped != "2026-03-08T01:30:00-05:00" {
		t.Fatalf("02:30 on March 8 = %s", skipped)
	}

	// Autumn: the day is 25 hours long; 01:30 happens twice.
	fallNoon := newTime(t, ints(2026, 11, 1, 12), ny)
	fallStart := mustMethod(t, fallNoon, "start_of_day")
	if d, _ := fallNoon.Minus(fallStart); d != object.Float(13*3600) {
		t.Fatalf("midnight to noon on November 1 = %v", d)
	}
	firstHalf := mustCall(t, "parse", object.Args{str("2026-11-01T01:30:00-04:00")})
	secondHalf, _ := firstHalf.Add(object.Integer(3600))
	inNY := mustMethod(t, secondHalf, "to_zone", str("America/New_York"))
	if text(inNY) != "2026-11-01T01:30:00-05:00" {
		t.Fatalf("second 01:30 = %s", text(inNY))
	}
	repeated := newTime(t, ints(2026, 11, 1, 1, 30), ny).String()
	if repeated != "2026-11-01T01:30:00-04:00" && repeated != "2026-11-01T01:30:00-05:00" {
		t.Fatalf("01:30 on November 1 = %s", repeated)
	}

	// Where the clocks jump over midnight, the day starts when they jump:
	// 00:00 does not exist, and the start is still on the same calendar day.
	for zone, want := range map[string]string{
		"America/Santiago": "2026-09-06T01:00:00-03:00",
		"Africa/Cairo":     "2026-04-24T01:00:00+03:00",
	} {
		day := ints(2026, 9, 6, 12)
		if zone == "Africa/Cairo" {
			day = ints(2026, 4, 24, 12)
		}
		start := mustMethod(t, newTime(t, day, kw("zone", str(zone))), "start_of_day")
		if text(start) != want {
			t.Errorf("start_of_day in %s = %s, want %s", zone, text(start), want)
		}
	}
}

func TestRollover(t *testing.T) {
	utc := kw("zone", str("UTC"))
	cases := map[string]*timevalue.Time{
		"2027-01-01T00:00:00Z": newTime(t, ints(2026, 13, 1), utc),
		"2026-06-30T00:00:00Z": newTime(t, ints(2026, 7, 0), utc),
		"2026-03-01T00:00:00Z": newTime(t, ints(2026, 2, 29), utc),
		"2026-07-20T01:00:00Z": newTime(t, ints(2026, 7, 19, 25), utc),
	}
	for want, got := range cases {
		if got.String() != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
	// The result must be within the years 1 to 9999, and a part too large
	// to roll over is refused rather than wrapped around.
	edges := map[string]*timevalue.Time{
		"9999-12-31T23:59:59Z": newTime(t, ints(9999, 12, 31, 23, 59, 59), utc),
		"0001-01-01T00:00:00Z": newTime(t, ints(1, 1, 1), utc),
		"9999-12-31T00:00:00Z": newTime(t, ints(10000, 1, 0), utc),
	}
	for want, got := range edges {
		if got.String() != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
	for _, parts := range []object.Args{
		ints(10000, 1, 1), ints(0, 12, 31), ints(-1, 1, 1), ints(9999, 12, 31, 24),
		ints(2026, 9223372036854775807, 1), ints(2026, 1, -9223372036854775807-1),
		ints(2026, 1, 1, 0, 0, 9223372036854775807), ints(9223372036854775807, 1, 1),
	} {
		_, err := timevalue.Type.Function.Call(object.CallArgs{Positional: parts, Keyword: object.Kwargs{utc}})
		if err == nil || !errors.Is(err, object.ValueError) || err.Error() != "Time() result is out of range (years 1 to 9999)" {
			t.Errorf("Time%v = %v", parts, err)
		}
	}
	_, err := timevalue.Type.Function.Call(object.CallArgs{Positional: object.Args{object.Integer(2026), object.Float(1), object.Integer(1)}})
	if !errors.Is(err, object.TypeError) {
		t.Fatalf("Float month = %v", err)
	}
	_, err = timevalue.Type.Function.Call(object.CallArgs{Positional: ints(2026, 1, 1), Keyword: object.Kwargs{kw("nanosecond", object.Integer(1))}})
	if !errors.Is(err, object.TypeError) {
		t.Fatalf("nanosecond= is gone, got %v", err)
	}
}

func TestFormat(t *testing.T) {
	tm := newTime(t, ints(2026, 7, 5, 15, 4, 5), kw("zone", str("+08:00")))
	shifted, _ := tm.Add(object.Float(0.25))
	cases := map[string]string{
		"%Y-%m-%d %H:%M:%S":    "2026-07-05 15:04:05",
		"%y|%e|%I|%p|%j":       "26| 5|03|PM|186",
		"%A %a %B %b":          "Sunday Sun July Jul",
		"%z %Z":                "+0800 +08:00",
		"%G-W%V":               "2026-W27",
		"100%% at 2006 Jan %Y": "100% at 2006 Jan 2026",
		"":                     "",
		"no directives":        "no directives",
	}
	for layout, want := range cases {
		if got := text(mustMethod(t, tm, "format", str(layout))); got != want {
			t.Errorf("format(%q) = %q, want %q", layout, got, want)
		}
	}
	if got := text(mustMethod(t, shifted, "format", str("%S.%f"))); got != "05.250000" {
		t.Errorf("%%f = %s", got)
	}
	if got := text(mustMethod(t, shifted, "format")); got != "2026-07-05T15:04:05.25+08:00" {
		t.Errorf("format() = %s", got)
	}
	if got := text(mustMethod(t, shifted, "format", object.Nil)); got != text(shifted) {
		t.Errorf("format(nil) = %s", got)
	}
	// ISO week-numbering year differs from the calendar year at the edges.
	jan1 := newTime(t, ints(2027, 1, 1), kw("zone", str("UTC")))
	if got := text(mustMethod(t, jan1, "format", str("%G-W%V %Y"))); got != "2026-W53 2027" {
		t.Errorf("ISO week of 2027-01-01 = %s", got)
	}
	midnight := newTime(t, ints(2026, 7, 5, 0, 7), kw("zone", str("UTC")))
	if got := text(mustMethod(t, midnight, "format", str("%I %p"))); got != "12 AM" {
		t.Errorf("midnight = %s", got)
	}
	for layout, msg := range map[string]string{
		"%Q":   "format() unknown layout directive %Q",
		"50 %": "format() layout ends with a lone %",
	} {
		_, err := method(t, tm, "format", str(layout))
		if !errors.Is(err, object.ValueError) || err.Error() != msg {
			t.Errorf("format(%q) = %v", layout, err)
		}
	}
}

func TestParseISO(t *testing.T) {
	cases := map[string]string{
		"2026-07-19":                     "2026-07-19T00:00:00-04:00",
		"2026-07-19T09:30":               "2026-07-19T09:30:00-04:00",
		"2026-07-19 09:30:15":            "2026-07-19T09:30:15-04:00",
		"2026-07-19T09:30:15.250":        "2026-07-19T09:30:15.25-04:00",
		"2026-07-19T09:30Z":              "2026-07-19T09:30:00Z",
		"2026-07-19T09:30:15+08:00":      "2026-07-19T09:30:15+08:00",
		"2026-07-19T09:30:15.5-03:30":    "2026-07-19T09:30:15.5-03:30",
		"2026-07-19T09:30:15.123456789Z": "2026-07-19T09:30:15.123456789Z",
	}
	for in, want := range cases {
		got := mustCall(t, "parse", object.Args{str(in)}, kw("zone", str("America/New_York")))
		if text(got) != want {
			t.Errorf("parse(%q) = %s, want %s", in, text(got), want)
		}
	}
	if zone := attr(t, mustCall(t, "parse", object.Args{str("2026-07-19T13:30:00Z")}), "zone"); zone != str("UTC") {
		t.Errorf("Z gives zone %v", zone)
	}
	if zone := attr(t, mustCall(t, "parse", object.Args{str("2026-07-19T13:30:00+08:00")}, kw("zone", str("UTC"))), "zone"); zone != str("+08:00") {
		t.Errorf("an offset in the text wins over zone=, got %v", zone)
	}
	for _, bad := range []string{"19/07", "2026-7-19", "2026-07-19T", "2026-07-19T9:30", "2026-02-30", "2026-07-19T24:00", "2026-07-19 09:30 ", "2026-07-19T09:30:15+0800x"} {
		_, err := call("parse", object.Args{str(bad)})
		var e *object.Error
		if !errors.As(err, &e) || !errors.Is(err, object.ParseError) {
			t.Errorf("parse(%q) = %v", bad, err)
			continue
		}
		if e.Error() != "parse() "+quote(bad)+" is not an ISO 8601 time" {
			t.Errorf("message = %s", e.Error())
		}
		if v, _ := e.Field("text"); v != str(bad) {
			t.Errorf("text = %v", v)
		}
		if v, ok := e.Field("layout"); !ok || v != object.Nil {
			t.Errorf("layout = %v, %v", v, ok)
		}
	}
}

func quote(s string) string { return `"` + s + `"` }

func TestParseLayout(t *testing.T) {
	utc := kw("zone", str("UTC"))
	cases := []struct{ text, layout, want string }{
		{"19/07/2026 14:05", "%d/%m/%Y %H:%M", "2026-07-19T14:05:00Z"},
		{"9/7/26", "%d/%m/%y", "2026-07-09T00:00:00Z"},
		{"1/1/70", "%d/%m/%y", "1970-01-01T00:00:00Z"},
		{"Sun, 19 Jul 2026 13:30:00 GMT", "%a, %d %b %Y %H:%M:%S GMT", "2026-07-19T13:30:00Z"},
		{"sunday JULY 19 2026", "%A %B %d %Y", "2026-07-19T00:00:00Z"},
		{"03:15 pm", "%I:%M %p", "1900-01-01T15:15:00Z"},
		{"12:00 AM", "%I:%M %p", "1900-01-01T00:00:00Z"},
		{"2026-07-19 09:30:15.25", "%Y-%m-%d %H:%M:%S.%f", "2026-07-19T09:30:15.25Z"},
		{"2026-07-19 +0800", "%Y-%m-%d %z", "2026-07-19T00:00:00+08:00"},
		{"2026-07-19 -03:30", "%Y-%m-%d %z", "2026-07-19T00:00:00-03:30"},
		{"2026-07-19 Z", "%Y-%m-%d %z", "2026-07-19T00:00:00Z"},
		{"day 200 of 2026-07-19", "day %j of %Y-%m-%d", "2026-07-19T00:00:00Z"},
		{"100% 2026", "100%% %Y", "2026-01-01T00:00:00Z"},
		{" 9.7.2026", "%e.%m.%Y", "2026-07-09T00:00:00Z"},
	}
	for _, c := range cases {
		got, err := call("parse", object.Args{str(c.text)}, kw("layout", str(c.layout)), utc)
		if err != nil {
			t.Errorf("parse(%q, %q) error %v", c.text, c.layout, err)
			continue
		}
		if text(got) != c.want {
			t.Errorf("parse(%q, %q) = %s, want %s", c.text, c.layout, text(got), c.want)
		}
	}
	if zone := attr(t, mustCall(t, "parse", object.Args{str("2026-07-19 +0800")}, kw("layout", str("%Y-%m-%d %z"))), "zone"); zone != str("+08:00") {
		t.Errorf("%%z zone = %v", zone)
	}
	for _, c := range []struct{ text, layout string }{
		{"19/07", "%d/%m/%Y"}, {"31/06/2026", "%d/%m/%Y"}, {"2026", "%Y-%m"}, {"13:00 PM", "%I:%M %p"},
		{"26", "%Y"}, {"2026-07-19x", "%Y-%m-%d"}, {"2026-13-01", "%Y-%m-%d"},
	} {
		_, err := call("parse", object.Args{str(c.text)}, kw("layout", str(c.layout)))
		var e *object.Error
		if !errors.As(err, &e) || !errors.Is(err, object.ParseError) {
			t.Errorf("parse(%q, %q) = %v", c.text, c.layout, err)
			continue
		}
		if want := "parse() " + quote(c.text) + " does not match layout " + quote(c.layout); e.Error() != want {
			t.Errorf("message = %s", e.Error())
		}
		if v, _ := e.Field("layout"); v != str(c.layout) {
			t.Errorf("layout = %v", v)
		}
	}
	for layout, msg := range map[string]string{
		"%Y %Z": "parse() layout directive %Z can only be used to format",
		"%G-%V": "parse() layout directive %G can only be used to format",
		"%Y %q": "parse() unknown layout directive %q",
	} {
		// The layout is checked whatever the text.
		_, err := call("parse", object.Args{str("nothing like it")}, kw("layout", str(layout)))
		if !errors.Is(err, object.ValueError) || err.Error() != msg {
			t.Errorf("parse layout %q = %v", layout, err)
		}
	}
}

func TestArithmetic(t *testing.T) {
	start := newTime(t, ints(2026, 7, 19, 9, 30), kw("zone", str("Europe/Paris")))
	later, _ := start.Add(object.Integer(90))
	if text(later) != "2026-07-19T09:31:30+02:00" || attr(t, later, "zone") != str("Europe/Paris") {
		t.Fatalf("t + 90 = %s", text(later))
	}
	left, ok, _ := start.RAdd(object.Float(1.5))
	if !ok || text(left) != "2026-07-19T09:30:01.5+02:00" {
		t.Fatalf("1.5 + t = %v", left)
	}
	earlier, _ := start.Minus(object.Integer(3600))
	if attr(t, earlier, "hour") != object.Integer(8) {
		t.Fatal("t - 3600")
	}
	diff, _ := later.(*timevalue.Time).Minus(start)
	if diff != object.Float(90) {
		t.Fatalf("t - u = %v", diff)
	}
	far := newTime(t, ints(9000, 1, 1), kw("zone", str("UTC")))
	near := newTime(t, ints(1, 1, 1), kw("zone", str("UTC")))
	if d, _ := far.Minus(near); d != object.Float(283980988800) {
		t.Fatalf("t - u beyond Duration = %v", d)
	}
	if _, err := start.Add(object.Integer(1 << 62)); !errors.Is(err, object.ValueError) {
		t.Fatalf("out of range = %v", err)
	}
	// One range for every Time: the years 1 to 9999, what unix accepts.
	for _, seconds := range []object.Object{object.Float(1e18), object.Integer(300000000000), object.Integer(-70000000000)} {
		if _, err := start.Add(seconds); err == nil || err.Error() != "Time arithmetic result is out of range (years 1 to 9999)" {
			t.Errorf("t + %v = %v", seconds, err)
		}
	}
	if _, err := far.Minus(object.Integer(-40000000000)); err == nil || !errors.Is(err, object.ValueError) {
		t.Errorf("t - big = %v", err)
	}
	if _, err := start.Add(str("soon")); !errors.Is(err, object.TypeError) {
		t.Fatalf("t + String = %v", err)
	}
	// Equality and ordering are by instant, whatever the zones.
	utc := mustMethod(t, start, "to_utc")
	if !modtest.Equal(utc, start) {
		t.Fatal("same instant in two zones is not equal")
	}
	h1, _ := utc.(*timevalue.Time).Hash()
	h2, _ := start.Hash()
	if h1 != h2 {
		t.Fatal("hash disagrees with equality")
	}
	if c, _ := start.Compare(later); c != -1 {
		t.Fatal("ordering")
	}
}

func TestUnixNowSleep(t *testing.T) {
	epoch := mustCall(t, "unix", ints(0))
	if attr(t, epoch, "zone") != str("local") || text(mustMethod(t, epoch, "to_utc")) != "1970-01-01T00:00:00Z" {
		t.Fatalf("unix(0) = %s", text(epoch))
	}
	for _, bad := range []object.Object{object.Integer(1 << 50), object.Float(1e300)} {
		if _, err := call("unix", object.Args{bad}); !errors.Is(err, object.ValueError) {
			t.Errorf("unix(%v) = %v", bad, err)
		}
	}
	a := mustCall(t, "now", nil)
	b := mustCall(t, "now", nil)
	if d, _ := b.(*timevalue.Time).Minus(a); d.(object.Float) < 0 {
		t.Fatal("now() went backwards")
	}
	if attr(t, a, "zone") != str("local") {
		t.Fatal("now() is local")
	}
	if v := mustCall(t, "sleep", ints(0)); v != object.Nil {
		t.Fatal("sleep(0)")
	}
	if _, err := call("sleep", object.Args{object.Float(-1)}); err == nil || err.Error() != "sleep() argument 'seconds' must be non-negative, got -1" {
		t.Fatalf("sleep(-1) = %v", err)
	}
	if _, err := call("sleep", object.Args{str("1")}); !errors.Is(err, object.TypeError) {
		t.Fatalf("sleep(String) = %v", err)
	}
}

// The zone database is linked by programs that import time and by no
// other: fs shares the Time type but must not pull it in.
func TestTZDataLinkedOnlyByTime(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not available")
	}
	deps := func(pkg string) string {
		out, err := exec.Command("go", "list", "-deps", pkg).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", pkg, err)
		}
		return string(out)
	}
	if !strings.Contains(deps("github.com/aisk/goblin/extension/time"), "time/tzdata\n") {
		t.Fatal("extension/time does not link time/tzdata")
	}
	for _, pkg := range []string{"github.com/aisk/goblin/extension/fs", "github.com/aisk/goblin/extension/internal/timevalue"} {
		if strings.Contains(deps(pkg), "time/tzdata\n") {
			t.Errorf("%s links time/tzdata", pkg)
		}
	}
	// Loading a zone does not depend on the process's own zone.
	if _, err := stdtime.LoadLocation("America/New_York"); err != nil {
		t.Fatal(err)
	}
}
