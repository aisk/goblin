package time

import (
	"hash/maphash"
	"math"
	stdtime "time"

	"github.com/aisk/goblin/object"
)

// hashSeed keys Time hashes. Hashes never leave the process, so a per-process
// seed is fine.
var hashSeed = maphash.MakeSeed()

// Time wraps Go's time.Time as a goblin object.
type Time struct {
	object.OpaqueBase
	Value stdtime.Time
}

func NewTime(t stdtime.Time) *Time {
	return &Time{OpaqueBase: object.MakeOpaqueBase("Time"), Value: t}
}

func (t *Time) String() string {
	return t.Value.Format(stdtime.RFC3339)
}

func (t *Time) ToString() (string, error) { return t.String(), nil }

func (t *Time) ToBool() (bool, error) { return !t.Value.IsZero(), nil }

func (t *Time) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Time)
	return ok && t.Value.Equal(v.Value), nil
}

// Hash agrees with Equals, which compares instants regardless of location.
func (t *Time) Hash() (uint64, error) {
	return maphash.Comparable(hashSeed, [2]int64{t.Value.Unix(), int64(t.Value.Nanosecond())}), nil
}

func (t *Time) Compare(other object.Object) (int, error) {
	ot, ok := other.(*Time)
	if !ok {
		return 0, object.NewTypeError("cannot compare Time with %s", other.TypeName())
	}
	if t.Value.Before(ot.Value) {
		return -1, nil
	}
	if t.Value.After(ot.Value) {
		return 1, nil
	}
	return 0, nil
}

// Arithmetic follows the module's convention that a duration is a number of
// seconds: t + seconds and t - seconds shift a time, and t - u is the Float
// number of seconds from u to t.
func (t *Time) Add(other object.Object) (object.Object, error) {
	if result, ok, err := t.shift(other, 1); ok {
		return result, err
	}
	return nil, object.NewTypeError("cannot add Time and %s", other.TypeName())
}

func (t *Time) RAdd(left object.Object) (object.Object, bool, error) {
	return t.shift(left, 1)
}

func (t *Time) Minus(other object.Object) (object.Object, error) {
	if u, ok := other.(*Time); ok {
		seconds := float64(t.Value.Unix()) - float64(u.Value.Unix())
		nanos := t.Value.Nanosecond() - u.Value.Nanosecond()
		return object.Float(seconds + float64(nanos)/float64(stdtime.Second)), nil
	}
	if result, ok, err := t.shift(other, -1); ok {
		return result, err
	}
	return nil, object.NewTypeError("cannot subtract Time and %s", other.TypeName())
}

// maxShiftUnix bounds the Unix seconds a shifted time may reach. Go's time
// package stores seconds since year 1, so values near the int64 limits would
// overflow inside time.Unix.
const maxShiftUnix = math.MaxInt64 / 2

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

func (t *Time) GetAttr(name string) (object.Object, error) {
	if value, ok := timeType.Attribute(name); ok {
		return value, nil
	}
	switch name {
	case "attributes":
		return object.AttributesFunction(t), nil
	// Date components
	case "year":
		return object.Integer(t.Value.Year()), nil
	case "month":
		return object.Integer(t.Value.Month()), nil
	case "day":
		return object.Integer(t.Value.Day()), nil
	case "hour":
		return object.Integer(t.Value.Hour()), nil
	case "minute":
		return object.Integer(t.Value.Minute()), nil
	case "second":
		return object.Integer(t.Value.Second()), nil
	case "nanosecond":
		return object.Integer(t.Value.Nanosecond()), nil

	// Unix timestamps
	case "unix":
		return object.Integer(t.Value.Unix()), nil
	case "unix_nano":
		return object.Integer(t.Value.UnixNano()), nil

	// Weekday
	case "weekday":
		return object.String(t.Value.Weekday().String()), nil

	// Methods — return closures that capture the receiver
	case "elapsed":
		return &object.Function{
			Name: "elapsed",
			Fn: func(args object.CallArgs) (object.Object, error) {
				if err := object.NewArgParser("elapsed", args).Finish(); err != nil {
					return nil, err
				}
				d := stdtime.Since(t.Value)
				return object.Float(float64(d) / float64(stdtime.Second)), nil
			},
		}, nil
	case "format":
		return &object.Function{
			Name: "format",
			Fn: func(args object.CallArgs) (object.Object, error) {
				p := object.NewArgParser("format", args)
				layout := p.Str("layout")
				if err := p.Finish(); err != nil {
					return nil, err
				}
				return object.String(t.Value.Format(string(layout))), nil
			},
		}, nil

	default:
		return nil, object.NewAttributeError("Time has no attribute '%s'", name)
	}
}

func (t *Time) Attributes() []string {
	return timeType.Attributes("attributes", "year", "month", "day", "hour", "minute", "second", "nanosecond", "unix", "unix_nano", "weekday", "elapsed", "format")
}

var _ object.Object = (*Time)(nil)
