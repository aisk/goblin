package time

import (
	stdtime "time"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{
		Name: "time",
		Members: map[string]object.Object{
			"Time":  &object.Function{Name: "Time", Fn: timeConstructor},
			"now":   &object.Function{Name: "now", Fn: now},
			"sleep": &object.Function{Name: "sleep", Fn: sleep},
			"parse": &object.Function{Name: "parse", Fn: parse},
			"unix":  &object.Function{Name: "unix", Fn: unix},
		},
	}, nil
}

// timeConstructor builds a Time from calendar components in local time.
func timeConstructor(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Time", args)
	year := p.Int("year")
	month := p.Int("month")
	day := p.Int("day")
	hour := p.IntOr("hour", 0)
	minute := p.IntOr("minute", 0)
	second := p.IntOr("second", 0)
	nanosecond := p.IntOr("nanosecond", 0)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	t := stdtime.Date(int(year), stdtime.Month(month), int(day), int(hour), int(minute), int(second), int(nanosecond), stdtime.Local)
	return NewTime(t), nil
}

// now returns the current local time.
func now(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("now", args).Finish(); err != nil {
		return nil, err
	}
	return NewTime(stdtime.Now()), nil
}

// sleep pauses execution for the given number of seconds (float).
func sleep(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("sleep", args)
	seconds := p.Float64("seconds")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	stdtime.Sleep(stdtime.Duration(seconds * float64(stdtime.Second)))
	return object.Nil, nil
}

// parse parses a formatted string and returns the time value it represents.
// Uses Go's reference layout (e.g. "2006-01-02").
func parse(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("parse", args)
	layout, value := p.Str("layout"), p.Str("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	t, err := stdtime.Parse(string(layout), string(value))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "parse() invalid time string", err)
	}
	return NewTime(t), nil
}

// unix returns the local time corresponding to the given Unix time.
func unix(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("unix", args)
	sec := p.Int("seconds")
	nsec := p.IntOr("nanoseconds", 0)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return NewTime(stdtime.Unix(int64(sec), int64(nsec))), nil
}
