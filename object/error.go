package object

import "fmt"

type Error struct {
	Value string
}

func NewError(value string) *Error {
	return &Error{Value: value}
}

func (e *Error) Repr() string {
	return fmt.Sprintf("object.Error(%s)", e.String())
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

var _ Object = (*Error)(nil)
var _ error = (*Error)(nil)

var NotImplementedError = NewError("not implemented")
