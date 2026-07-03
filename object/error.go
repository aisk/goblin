package object

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
)

type Error struct {
	Value string
	// Wrapped is the underlying error this one wraps, or nil. Typed as the Go
	// `error` interface so the standard library's errors.Is / errors.Unwrap can
	// traverse the chain directly (see (*Error).Unwrap below); the `errors`
	// module's helpers delegate to them rather than reimplementing traversal.
	Wrapped error
}

var _ Object = (*Error)(nil)

func NewError(value string) *Error {
	return &Error{Value: value}
}

func NewSentinelError(value string, parent *Error) *Error {
	return &Error{Value: value, Wrapped: parent}
}

// NewWrappedError builds an error carrying message that wraps cause, following
// Go's convention where the rendered message is "message: cause".
func NewWrappedError(message string, cause error) *Error {
	return &Error{Value: message + ": " + cause.Error(), Wrapped: cause}
}

func WrapError(base *Error, message string, cause error) *Error {
	return &Error{Value: fmt.Sprintf("%s: %s", message, cause.Error()), Wrapped: typedCause{cause: cause, base: base}}
}

type typedCause struct {
	cause error
	base  *Error
}

func (e typedCause) Error() string {
	return e.cause.Error()
}

func (e typedCause) Unwrap() []error {
	return []error{e.cause, e.base}
}

// Unwrap exposes the wrapped error to the standard library's errors.Is /
// errors.Unwrap, making *Error a first-class participant in Go error chains.
func (e *Error) Unwrap() error {
	return e.Wrapped
}

func (e *Error) String() string {
	return e.Value
}

func (e *Error) Bool() bool {
	return true
}

func (e *Error) Compare(other Object) (int, error) {
	return 0, NewTypeError("cannot compare Error and %T", other)
}

func (e *Error) Add(other Object) (Object, error) {
	return nil, NewTypeError("cannot add Error and %T", other)
}

func (e *Error) Minus(other Object) (Object, error) {
	return nil, NewTypeError("cannot subtract Error and %T", other)
}

func (e *Error) Multiply(other Object) (Object, error) {
	return nil, NewTypeError("cannot multiply Error and %T", other)
}

func (e *Error) Divide(other Object) (Object, error) {
	return nil, NewTypeError("cannot divide Error and %T", other)
}

func (e *Error) And(other Object) (Object, error) {
	return nil, NewTypeError("cannot perform AND operation on Error and %T", other)
}

func (e *Error) Or(other Object) (Object, error) {
	return nil, NewTypeError("cannot perform OR operation on Error and %T", other)
}

func (e *Error) Not() (Object, error) {
	return nil, NewTypeError("cannot perform NOT operation on Error")
}

func (e *Error) Iter() ([]Object, error) {
	return nil, NewTypeError("Error does not support iteration")
}

func (e *Error) Index(index Object) (Object, error) {
	return nil, NewTypeError("Error is not indexable")
}

func (e *Error) Error() string {
	return e.Value
}

func (e *Error) GetAttr(name string) (Object, error) {
	switch name {
	case "message":
		return String(e.Value), nil
	case "wrap":
		return &Function{Name: "wrap", Fn: e.Wrap}, nil
	case "unwrap":
		return &Function{Name: "unwrap", Fn: e.Unwrapped}, nil
	case "is":
		return &Function{Name: "is", Fn: e.Is}, nil
	case "constructor":
		return ErrorConstructorFn, nil
	}
	return nil, NewAttributeError("Error has no attribute '%s'", name)
}

// Wrap returns a new Error that carries message and wraps the receiver as its
// cause, mirroring Go's fmt.Errorf("message: %w", err). Usage: err.wrap("msg").
func (e *Error) Wrap(args CallArgs) (Object, error) {
	bound, err := BindArguments("wrap", []string{"message"}, "", "", args)
	if err != nil {
		return nil, err
	}
	message, ok := bound["message"].(String)
	if !ok {
		return nil, NewTypeError("wrap() argument must be a string, got %T", bound["message"])
	}
	return NewWrappedError(string(message), e), nil
}

// Unwrapped returns the immediate cause of the receiver, or Nil if it wraps
// nothing. Usage: err.unwrap(). Traversal is delegated to the standard library.
func (e *Error) Unwrapped(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("unwrap", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("unwrap() takes no arguments, got %d", len(args.Positional))
	}
	cause := errors.Unwrap(e)
	if cause == nil {
		return Nil, nil
	}
	if obj, ok := cause.(Object); ok {
		return obj, nil
	}
	// A non-Object Go error can only appear if native code injected one; present
	// it to Goblin as a plain Error.
	return NewError(cause.Error()), nil
}

// Is reports whether target appears anywhere in the receiver's cause chain,
// delegating to the standard library's errors.Is. Usage: err.is(target).
func (e *Error) Is(args CallArgs) (Object, error) {
	bound, err := BindArguments("is", []string{"target"}, "", "", args)
	if err != nil {
		return nil, err
	}
	target, ok := bound["target"].(*Error)
	if !ok {
		return nil, NewTypeError("is() argument must be an Error, got %T", bound["target"])
	}
	return Bool(errors.Is(e, target)), nil
}

var _ error = (*Error)(nil)

// Predefined error values covering the common failure kinds. Each is a distinct
// sentinel that can be raised, given context with .wrap(), matched with .is(),
// or used as the base for a derived error.
var (
	BaseError       = NewError("Error")
	TypeError       = NewSentinelError("TypeError", BaseError)
	ValueError      = NewSentinelError("ValueError", BaseError)
	LookupError     = NewSentinelError("LookupError", BaseError)
	ArithmeticError = NewSentinelError("ArithmeticError", BaseError)
	IOError         = NewSentinelError("IOError", BaseError)

	ParseError        = NewSentinelError("ParseError", ValueError)
	IndexError        = NewSentinelError("IndexError", LookupError)
	KeyError          = NewSentinelError("KeyError", LookupError)
	AttributeError    = NewSentinelError("AttributeError", LookupError)
	NameError         = NewSentinelError("NameError", LookupError)
	ImportError       = NewSentinelError("ImportError", LookupError)
	ZeroDivisionError = NewSentinelError("ZeroDivisionError", ArithmeticError)

	NotExistError   = NewSentinelError("NotExistError", IOError)
	ExistError      = NewSentinelError("ExistError", IOError)
	PermissionError = NewSentinelError("PermissionError", IOError)
	TimeoutError    = NewSentinelError("TimeoutError", IOError)
	NetworkError    = NewSentinelError("NetworkError", IOError)

	NotImplementedError = NewSentinelError("NotImplementedError", BaseError)
)

// typedError builds an Error whose message is the formatted string and whose
// cause is base, so the runtime's own failures are matchable with .is(base)
// while the rendered message stays exactly what it was.
func typedError(base *Error, format string, a ...any) *Error {
	return &Error{Value: fmt.Sprintf(format, a...), Wrapped: base}
}

// NewTypeError, NewValueError, NewIndexError, NewKeyError and
// NewZeroDivisionError create errors tagged with the matching sentinel. They are
// used throughout the runtime in place of fmt.Errorf so that raised failures can
// be caught by kind.
func NewTypeError(format string, a ...any) *Error { return typedError(TypeError, format, a...) }
func NewValueError(format string, a ...any) *Error {
	return typedError(ValueError, format, a...)
}
func NewIndexError(format string, a ...any) *Error {
	return typedError(IndexError, format, a...)
}
func NewKeyError(format string, a ...any) *Error { return typedError(KeyError, format, a...) }
func NewZeroDivisionError(format string, a ...any) *Error {
	return typedError(ZeroDivisionError, format, a...)
}
func NewAttributeError(format string, a ...any) *Error {
	return typedError(AttributeError, format, a...)
}
func NewNameError(format string, a ...any) *Error {
	return typedError(NameError, format, a...)
}
func NewImportError(format string, a ...any) *Error {
	return typedError(ImportError, format, a...)
}
func NewParseError(format string, a ...any) *Error {
	return typedError(ParseError, format, a...)
}
func NewIOError(format string, a ...any) *Error { return typedError(IOError, format, a...) }
func NewNotExistError(format string, a ...any) *Error {
	return typedError(NotExistError, format, a...)
}
func NewExistError(format string, a ...any) *Error {
	return typedError(ExistError, format, a...)
}
func NewPermissionError(format string, a ...any) *Error {
	return typedError(PermissionError, format, a...)
}
func NewTimeoutError(format string, a ...any) *Error {
	return typedError(TimeoutError, format, a...)
}
func NewNetworkError(format string, a ...any) *Error {
	return typedError(NetworkError, format, a...)
}

func ErrorKind(err error, fallback *Error) *Error {
	if errors.Is(err, fs.ErrNotExist) {
		return NotExistError
	}
	if errors.Is(err, fs.ErrExist) {
		return ExistError
	}
	if errors.Is(err, fs.ErrPermission) {
		return PermissionError
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return TimeoutError
		}
		return NetworkError
	}
	return fallback
}

func WrapNativeError(fallback *Error, message string, err error) *Error {
	return WrapError(ErrorKind(err, fallback), message, err)
}

var ErrorConstructorFn = &Function{Name: "Error", Fn: ErrorConstructor}

func ErrorConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Error", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return NewError(""), nil
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("Error() takes at most 1 argument, got %d", len(args.Positional))
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
	return NewTypeError("raise expects an Error, got: %s", v.String())
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
