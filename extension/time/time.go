// Package time is the time module: instants, the clock, time zones and
// sleeping. An instant is a Time; a duration is a number of seconds.
package time

import (
	"math"
	stdtime "time"
	// The zone database, so IANA names work where the system has none.
	// Only programs that import time link it.
	_ "time/tzdata"

	"github.com/aisk/goblin/extension/internal/timevalue"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("time", map[string]object.Object{
		"Time":   timevalue.Type.Function,
		"MINUTE": object.Integer(60),
		"HOUR":   object.Integer(3600),
		"DAY":    object.Integer(86400),
	}, object.Funcs{
		"now":   now,
		"sleep": sleep,
		"parse": parse,
		"unix":  unix,
	}), nil
}

// now returns the current instant in the local zone, keeping Go's monotonic
// reading so the difference of two now() values is measured on it.
func now(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("now", args); err != nil {
		return nil, err
	}
	return timevalue.NewTime(stdtime.Now()), nil
}

// seconds reads a non-negative number of seconds as a duration.
func seconds(fn string, p *object.ArgParser) (float64, error) {
	s := p.Float64("seconds")
	if err := p.Finish(); err != nil {
		return 0, err
	}
	if s < 0 || math.IsNaN(s) {
		return 0, object.NegativeArgumentError(fn, "seconds", s)
	}
	return s, nil
}

// sleep pauses the calling goblin.
func sleep(args object.CallArgs) (object.Object, error) {
	s, err := seconds("sleep", object.NewArgParser("sleep", args))
	if err != nil {
		return nil, err
	}
	if s >= float64(math.MaxInt64)/float64(stdtime.Second) {
		stdtime.Sleep(math.MaxInt64)
	} else {
		stdtime.Sleep(stdtime.Duration(s * float64(stdtime.Second)))
	}
	return object.Nil, nil
}

// parse reads text per a strftime layout, or as ISO 8601 without one; text
// without an offset is wall-clock time in zone.
func parse(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse", args)
	text := p.Str("text")
	layout, hasLayout := p.NullableStr("layout")
	zone, hasZone := p.NullableStr("zone")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	loc, err := timevalue.ZoneOr("parse", zone, hasZone)
	if err != nil {
		return nil, err
	}
	return timevalue.Parse("parse", string(text), layout, hasLayout, loc)
}

// minUnix and maxUnix bound unix(): the years Go's time.Time can hold.
const (
	minUnix = -62135596800 // 0001-01-01T00:00:00Z
	maxUnix = 253402300799 // 9999-12-31T23:59:59Z
)

// unix is the instant seconds after the Unix epoch, in the local zone.
func unix(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("unix", args)
	value := p.Number("seconds")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var t stdtime.Time
	switch v := value.(type) {
	case object.Integer:
		if v < minUnix || v > maxUnix {
			return nil, errUnixRange(value)
		}
		t = stdtime.Unix(int64(v), 0)
	case object.Float:
		f := float64(v)
		if math.IsNaN(f) || f < minUnix || f >= maxUnix+1 {
			return nil, errUnixRange(value)
		}
		whole := math.Floor(f)
		t = stdtime.Unix(int64(whole), int64(math.Round((f-whole)*1e9)))
	}
	return timevalue.NewTime(t), nil
}

func errUnixRange(v object.Object) error {
	text, _ := v.ToString()
	return object.NewValueError("unix() argument 'seconds' is out of range, got %s", text)
}
