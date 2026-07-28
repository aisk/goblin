package time

import (
	stdtime "time"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"now":   &object.Function{Name: "now", Fn: now},
			"sleep": &object.Function{Name: "sleep", Fn: sleep},
			"parse": &object.Function{Name: "parse", Fn: parse},
			"unix":  &object.Function{Name: "unix", Fn: unix},
			"since": &object.Function{Name: "since", Fn: since},
		},
	}, nil
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
	seconds := p.Number("seconds")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	switch v := seconds.(type) {
	case object.Float:
		stdtime.Sleep(stdtime.Duration(float64(v) * float64(stdtime.Second)))
	case object.Integer:
		stdtime.Sleep(stdtime.Duration(int64(v)) * stdtime.Second)
	}
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
		return nil, object.WrapError(object.ParseError, "parse() failed", err)
	}
	return NewTime(t), nil
}

// unix returns the local time corresponding to the given Unix time, in seconds.
func unix(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("unix", args)
	sec := p.Int("seconds")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return NewTime(stdtime.Unix(int64(sec), 0)), nil
}

// since returns the number of seconds (float) elapsed since the given Time.
func since(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("since", args)
	timeObj := p.Any("time")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	t, ok := timeObj.(*Time)
	if !ok {
		return nil, object.NewTypeError("since() argument must be a Time")
	}
	d := stdtime.Since(t.Value)
	return object.Float(float64(d) / float64(stdtime.Second)), nil
}
