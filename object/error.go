package object

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strings"
)

// Frame describes one Goblin-level stack frame. It deliberately contains
// only runtime-neutral data so both the interpreter and transpiled programs
// can attach frames without depending on the AST or token packages.
type Frame struct {
	Module   string
	Function string
	File     string
	Line     int
	Column   int
}

type Error struct {
	Value string
	// Wrapped is the underlying error this one wraps, or nil. Typed as the Go
	// `error` interface so the standard library's errors.Is / errors.Unwrap can
	// traverse the chain directly (see (*Error).Unwrap below); the `errors`
	// module's helpers delegate to them rather than reimplementing traversal.
	Wrapped error
	// Frames are ordered from the point where the error was first observed to
	// the outermost caller. WithFrame treats Error values as immutable.
	Frames []Frame
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

func (e *Error) ToString() (string, error) { return e.String(), nil }

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

// WithFrame returns an Error carrying one additional Goblin stack frame while
// retaining err in the Go error chain. In particular, errors.Is/errors.As and
// Goblin's Error.is() continue to see the original typed or native error.
func WithFrame(err error, frame Frame) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		frames := append([]Frame(nil), e.Frames...)
		frames = append(frames, frame)
		return &Error{Value: e.Value, Wrapped: e, Frames: frames}
	}
	return &Error{Value: err.Error(), Wrapped: err, Frames: []Frame{frame}}
}

// Traceback renders the language-level stack without exposing generated Go
// functions or the interpreter's own implementation stack.
func (e *Error) Traceback() string {
	if len(e.Frames) == 0 {
		return e.Value
	}
	var b strings.Builder
	b.WriteString("Traceback (most recent call last):\n")
	for i := len(e.Frames) - 1; i >= 0; i-- {
		f := e.Frames[i]
		name := f.Function
		if name == "" {
			name = "<module>"
		}
		fmt.Fprintf(&b, "  at %s", name)
		if f.Module != "" && f.Module != "main" {
			fmt.Fprintf(&b, " [%s]", f.Module)
		}
		if f.File != "" {
			fmt.Fprintf(&b, " (%s", f.File)
			if f.Line > 0 {
				fmt.Fprintf(&b, ":%d", f.Line)
				if f.Column > 0 {
					fmt.Fprintf(&b, ":%d", f.Column)
				}
			}
			b.WriteByte(')')
		}
		b.WriteByte('\n')
	}
	b.WriteString(e.Value)
	return b.String()
}

// Format keeps %v compatible with the historical short error message and
// makes %+v print the complete Goblin traceback, matching generated mains.
func (e *Error) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') {
		fmt.Fprint(s, e.Traceback())
		return
	}
	switch verb {
	case 'q':
		fmt.Fprintf(s, "%q", e.Value)
	default:
		fmt.Fprint(s, e.Value)
	}
}

func (e *Error) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(e), nil
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
	case "traceback":
		return &Function{Name: "traceback", Fn: e.TracebackValue}, nil
	}
	return nil, NewAttributeError("Error has no attribute '%s'", name)
}

func (e *Error) Attributes() []string {
	return []string{"attributes", "message", "wrap", "unwrap", "is", "constructor", "traceback"}
}

// TracebackValue exposes traceback formatting to Goblin as err.traceback().
func (e *Error) TracebackValue(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("traceback", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("traceback() takes no arguments, got %d", len(args.Positional))
	}
	return String(e.Traceback()), nil
}

// Wrap returns a new Error that carries message and wraps the receiver as its
// cause, mirroring Go's fmt.Errorf("message: %w", err). Usage: err.wrap("msg").
func (e *Error) Wrap(args CallArgs) (Object, error) {
	ap := NewArgParser("wrap", args)
	message := ap.Str("message")
	if err := ap.Finish(); err != nil {
		return nil, err
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
	ap := NewArgParser("is", args)
	target := ap.Any("target")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	t, ok := target.(*Error)
	if !ok {
		return nil, NewTypeError("is() argument must be an Error, got %T", target)
	}
	return Bool(errors.Is(e, t)), nil
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
	ap := NewArgParser("Error", args)
	message := ap.AnyOr("message", String(""))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	return NewError(message.String()), nil
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

// ErrorValue extracts the Goblin Error value carried by err, unwrapping any
// stack/cause wrappers added while the error propagated up the call stack.
// Errors that did not originate from `raise`
// (such as a runtime "division by zero") are surfaced as an *Error, so `catch`
// always binds an Error.
func ErrorValue(err error) Object {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if obj, ok := e.(Object); ok {
			return obj
		}
	}
	return NewError(err.Error())
}
