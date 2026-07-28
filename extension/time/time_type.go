package time

import (
	stdtime "time"

	"github.com/aisk/goblin/object"
)

// Time wraps Go's time.Time as a goblin object.
type Time struct {
	object.NoReflectedOps
	object.NoAssignment
	Value stdtime.Time
}

func NewTime(t stdtime.Time) *Time {
	return &Time{Value: t}
}

func (t *Time) TypeName() string { return "Time" }

func (t *Time) String() string {
	return t.Value.Format(stdtime.RFC3339)
}

func (t *Time) ToString() (string, error) { return t.String(), nil }

func (t *Time) ToBool() (bool, error) { return !t.Value.IsZero(), nil }

func (t *Time) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Time)
	return ok && t.Value.Equal(v.Value), nil
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

func (t *Time) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add Time")
}
func (t *Time) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract Time")
}
func (t *Time) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply Time")
}
func (t *Time) Divide(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot divide Time")
}
func (t *Time) Not() (object.Object, error) { return object.Bool(t.Value.IsZero()), nil }
func (t *Time) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Time does not support iteration")
}
func (t *Time) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Time is not indexable")
}

func (t *Time) GetAttr(name string) (object.Object, error) {
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
	return []string{"attributes", "year", "month", "day", "hour", "minute", "second", "nanosecond", "unix", "unix_nano", "weekday", "format"}
}

var _ object.Object = (*Time)(nil)
