package object

import (
	"errors"
	"fmt"
)

type Error struct {
	Value string
}

var _ Object = (*Error)(nil)

func NewError(value string) *Error {
	return &Error{Value: value}
}

func (e *Error) String() string {
	return e.Value
}

func (e *Error) Bool() bool {
	return true
}

func (e *Error) Compare(other Object) (int, error) {
	return 0, fmt.Errorf("cannot compare Error and %T", other)
}

func (e *Error) Add(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot add Error and %T", other)
}

func (e *Error) Minus(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot subtract Error and %T", other)
}

func (e *Error) Multiply(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot multiply Error and %T", other)
}

func (e *Error) Divide(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot divide Error and %T", other)
}

func (e *Error) And(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot perform AND operation on Error and %T", other)
}

func (e *Error) Or(other Object) (Object, error) {
	return nil, fmt.Errorf("cannot perform OR operation on Error and %T", other)
}

func (e *Error) Not() (Object, error) {
	return nil, fmt.Errorf("cannot perform NOT operation on Error")
}

func (e *Error) Iter() ([]Object, error) {
	return nil, fmt.Errorf("Error does not support iteration")
}

func (e *Error) Index(index Object) (Object, error) {
	return nil, fmt.Errorf("Error is not indexable")
}

func (e *Error) Error() string {
	return e.Value
}

func (e *Error) GetAttr(name string) (Object, error) {
	switch name {
	case "message":
		return String(e.Value), nil
	case "constructor":
		return ErrorConstructorFn, nil
	}
	return nil, fmt.Errorf("Error has no attribute '%s'", name)
}

var _ error = (*Error)(nil)

var NotImplementedError = NewError("not implemented")

var ErrorConstructorFn = &Function{Name: "Error", Fn: ErrorConstructor}

func ErrorConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Error", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return NewError(""), nil
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("Error() takes at most 1 argument, got %d", len(args.Positional))
	}
	return NewError(args.Positional[0].String()), nil
}

// Raise validates the value thrown by a `raise` statement. Only Error values
// may be raised; raising anything else is itself an error. It returns the
// error to propagate through the Go error channel that both backends use.
func Raise(v Object) error {
	if e, ok := v.(*Error); ok {
		return e
	}
	return NewError("raise expects an Error, got: " + v.String())
}

// ExcValue extracts the Goblin exception value carried by err, unwrapping any
// stack/cause wrappers (e.g. github.com/pkg/errors) added while the error
// propagated up the call stack. Errors that did not originate from `raise`
// (such as a runtime "division by zero") are surfaced as an *Error, so `catch`
// always binds an Error.
func ExcValue(err error) Object {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if obj, ok := e.(Object); ok {
			return obj
		}
	}
	return NewError(err.Error())
}
