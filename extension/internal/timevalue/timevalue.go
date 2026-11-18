// Package timevalue holds the Time value type shared by the modules that
// report instants (time, fs), so neither imports the other. The zone
// database is not linked here: extension/time imports time/tzdata, so a
// program that only reads fs.stat(...).modified_at does not carry it.
package timevalue

import (
	"hash/maphash"
	"math"
	stdtime "time"

	"github.com/aisk/goblin/object"
)

// hashSeed keys Time hashes. Hashes never leave the process, so a per-process
// seed is fine.
var hashSeed = maphash.MakeSeed()

// Time is an instant together with the zone it is shown in. It is
// immutable and, like every library value, always truthy.
type Time struct {
	object.OpaqueBase
	Value stdtime.Time
}

func NewTime(t stdtime.Time) *Time {
	return &Time{OpaqueBase: object.MakeOpaqueBase("Time"), Value: t}
}

// String is RFC 3339, with fractional seconds only when non-zero.
func (t *Time) String() string {
	return t.Value.Format(stdtime.RFC3339Nano)
}

func (t *Time) ToString() (string, error) { return t.String(), nil }

// PlainText satisfies object.PlainText: inside a collection a Time shows as
// Time("RFC 3339 text").
func (t *Time) PlainText() string { return t.String() }

func (t *Time) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Time)
	return ok && t.Value.Equal(v.Value), nil
}

// Hash agrees with Equals, which compares instants regardless of zone.
func (t *Time) Hash() (uint64, error) {
	return maphash.Comparable(hashSeed, [2]int64{t.Value.Unix(), int64(t.Value.Nanosecond())}), nil
}

func (t *Time) Compare(other object.Object) (int, error) {
	ot, ok := other.(*Time)
	if !ok {
		return 0, object.NewOperandError(object.OpCompare, "Time", other)
	}
	return t.Value.Compare(ot.Value), nil
}

// Arithmetic follows the convention that a duration is a number of seconds:
// t + seconds and t - seconds shift a time, and t - u is the Float number
// of seconds from u to t.
func (t *Time) Add(other object.Object) (object.Object, error) {
	if result, ok, err := t.shift(other, 1); ok {
		return result, err
	}
	return nil, object.NewOperandError(object.OpAdd, "Time", other)
}

func (t *Time) RAdd(left object.Object) (object.Object, bool, error) {
	return t.shift(left, 1)
}

func (t *Time) Minus(other object.Object) (object.Object, error) {
	if u, ok := other.(*Time); ok {
		// Sub reads the monotonic clock when both times carry it (two now()
		// values); it saturates beyond about 292 years, where the wall
		// clocks are subtracted instead.
		if d := t.Value.Sub(u.Value); d != math.MaxInt64 && d != math.MinInt64 {
			return object.Float(d.Seconds()), nil
		}
		seconds := float64(t.Value.Unix()) - float64(u.Value.Unix())
		nanos := t.Value.Nanosecond() - u.Value.Nanosecond()
		return object.Float(seconds + float64(nanos)/float64(stdtime.Second)), nil
	}
	if result, ok, err := t.shift(other, -1); ok {
		return result, err
	}
	return nil, object.NewOperandError(object.OpSubtract, "Time", other)
}

// maxShiftUnix bounds the Unix seconds a shifted time may reach. Go's time
// package stores seconds since year 1, so values near the int64 limits would
// overflow inside time.Unix.
const maxShiftUnix = math.MaxInt64 / 2

// maxDurationSeconds is the largest shift time.Duration holds exactly
// enough to go through Time.Add, which keeps the monotonic reading.
const maxDurationSeconds = math.MaxInt64 / int64(stdtime.Second)

// shift moves t by sign * seconds. ok is false when seconds is not a number,
// leaving the caller to report the operator's TypeError.
func (t *Time) shift(seconds object.Object, sign int64) (result object.Object, ok bool, err error) {
	var whole, nanos int64
	switch v := seconds.(type) {
	case object.Integer:
		if v < -maxShiftUnix || v > maxShiftUnix {
			return nil, true, errTimeRange()
		}
		whole = int64(v) * sign
	case object.Float:
		f := float64(v) * float64(sign)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, true, object.NewValueError("cannot shift Time by %v seconds", float64(v))
		}
		floor := math.Floor(f)
		if floor < -maxShiftUnix || floor > maxShiftUnix {
			return nil, true, errTimeRange()
		}
		whole = int64(floor)
		nanos = int64(math.Round((f - floor) * float64(stdtime.Second)))
	default:
		return nil, false, nil
	}
	if whole > -maxDurationSeconds && whole < maxDurationSeconds-1 {
		return NewTime(t.Value.Add(stdtime.Duration(whole)*stdtime.Second + stdtime.Duration(nanos))), true, nil
	}
	unix := t.Value.Unix() + whole
	if unix < -maxShiftUnix || unix > maxShiftUnix {
		return nil, true, errTimeRange()
	}
	shifted := stdtime.Unix(unix, int64(t.Value.Nanosecond())+nanos).In(t.Value.Location())
	return NewTime(shifted), true, nil
}

func errTimeRange() error {
	return object.NewValueError("Time arithmetic result is out of range")
}

// Constructor satisfies object.Constructed, so type_of answers the
// constructor the module exports.
func (t *Time) Constructor() *object.Function { return Type.Function }

func (t *Time) GetAttr(name string) (object.Object, error) {
	v := t.Value
	switch name {
	case "year":
		return object.Integer(v.Year()), nil
	case "month":
		return object.Integer(v.Month()), nil
	case "day":
		return object.Integer(v.Day()), nil
	case "hour":
		return object.Integer(v.Hour()), nil
	case "minute":
		return object.Integer(v.Minute()), nil
	case "second":
		return object.Integer(v.Second()), nil
	case "nanosecond":
		return object.Integer(v.Nanosecond()), nil
	case "weekday":
		// ISO 8601: Monday is 1, Sunday 7.
		return object.Integer((int(v.Weekday())+6)%7 + 1), nil
	case "unix":
		return object.Integer(v.Unix()), nil
	case "zone":
		return object.String(ZoneName(v.Location())), nil
	case "offset":
		_, offset := v.Zone()
		return object.Integer(offset), nil
	case "format":
		return &object.Function{Name: "format", Fn: t.format}, nil
	case "to_utc":
		return &object.Function{Name: "to_utc", Fn: t.toUTC}, nil
	case "to_zone":
		return &object.Function{Name: "to_zone", Fn: t.toZone}, nil
	case "start_of_day":
		return &object.Function{Name: "start_of_day", Fn: t.startOfDay}, nil
	}
	return nil, object.NewAttributeError("Time has no attribute '%s'", name)
}

func (t *Time) Attributes() []string {
	return []string{"year", "month", "day", "hour", "minute", "second", "nanosecond", "weekday", "unix", "zone", "offset", "format", "to_utc", "to_zone", "start_of_day"}
}

func (t *Time) format(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("format", args)
	layout, hasLayout := p.NullableStr("layout")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if !hasLayout {
		return object.String(t.String()), nil
	}
	text, err := Format("format", t.Value, string(layout))
	if err != nil {
		return nil, err
	}
	return object.String(text), nil
}

func (t *Time) toUTC(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("to_utc", args); err != nil {
		return nil, err
	}
	return NewTime(t.Value.In(stdtime.UTC)), nil
}

func (t *Time) toZone(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("to_zone", args)
	zone := p.Str("zone")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	loc, err := Zone("to_zone", string(zone))
	if err != nil {
		return nil, err
	}
	return NewTime(t.Value.In(loc)), nil
}

func (t *Time) startOfDay(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("start_of_day", args); err != nil {
		return nil, err
	}
	v := t.Value
	return NewTime(stdtime.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, v.Location())), nil
}

var _ object.Object = (*Time)(nil)

// Type is the one constructor identity of Time, exported by the time module.
var Type = object.NewNativeConstructor("Time", construct)

// construct builds a Time from wall-clock components in a zone, the local
// one by default. Out-of-range components roll over, as time.Date does.
func construct(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Time", args)
	year := p.Int("year")
	month := p.Int("month")
	day := p.Int("day")
	hour := p.IntOr("hour", 0)
	minute := p.IntOr("minute", 0)
	second := p.IntOr("second", 0)
	zone, hasZone := p.NullableStr("zone")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	loc, err := ZoneOr("Time", zone, hasZone)
	if err != nil {
		return nil, err
	}
	t := stdtime.Date(int(year), stdtime.Month(month), int(day), int(hour), int(minute), int(second), 0, loc)
	return NewTime(t), nil
}
