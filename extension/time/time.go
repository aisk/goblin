package time

import (
	"fmt"
	stdtime "time"

	"github.com/aisk/goblin/object"
	"github.com/pkg/errors"
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
	if err := object.RequireNoKeyword("now", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("now() requires no arguments")
	}
	return NewTime(stdtime.Now()), nil
}

// sleep pauses execution for the given number of seconds (float).
func sleep(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("sleep", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("sleep() requires exactly 1 argument")
	}
	switch v := args.Positional[0].(type) {
	case object.Float:
		stdtime.Sleep(stdtime.Duration(float64(v) * float64(stdtime.Second)))
	case object.Integer:
		stdtime.Sleep(stdtime.Duration(int64(v)) * stdtime.Second)
	default:
		return nil, fmt.Errorf("sleep() argument must be a number, got %T", args.Positional[0])
	}
	return object.Nil, nil
}

// parse parses a formatted string and returns the time value it represents.
// Uses Go's reference layout (e.g. "2006-01-02").
func parse(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("parse", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 2 {
		return nil, fmt.Errorf("parse() requires exactly 2 arguments")
	}
	layout, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, fmt.Errorf("parse() first argument must be a string")
	}
	value, ok := args.Positional[1].(object.String)
	if !ok {
		return nil, fmt.Errorf("parse() second argument must be a string")
	}
	t, err := stdtime.Parse(string(layout), string(value))
	if err != nil {
		return nil, errors.Wrap(err, "parse() failed")
	}
	return NewTime(t), nil
}

// unix returns the local time corresponding to the given Unix time, in seconds.
func unix(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("unix", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("unix() requires exactly 1 argument")
	}
	sec, ok := args.Positional[0].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("unix() argument must be an integer")
	}
	return NewTime(stdtime.Unix(int64(sec), 0)), nil
}

// since returns the number of seconds (float) elapsed since the given Time.
func since(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("since", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("since() requires exactly 1 argument")
	}
	t, ok := args.Positional[0].(*Time)
	if !ok {
		return nil, fmt.Errorf("since() argument must be a Time")
	}
	d := stdtime.Since(t.Value)
	return object.Float(float64(d) / float64(stdtime.Second)), nil
}
