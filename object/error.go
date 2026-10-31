package object

import (
	"errors"
	"fmt"
	"io/fs"
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
	inert[errorName]
	Value string
	// Wrapped is the underlying error this one wraps, or nil. Typed as the Go
	// `error` interface so the standard library's errors.Is / errors.Unwrap can
	// traverse the chain directly (see (*Error).Unwrap below); the `errors`
	// module's helpers delegate to them rather than reimplementing traversal.
	Wrapped error
	// frames is the Goblin stack the error unwound through, outermost caller
	// first. It is a shared linked list so that WithFrame adds a frame in
	// constant time: an error raised a hundred thousand calls deep (a
	// RecursionError) would otherwise copy its whole stack at every level.
	frames *frameNode
}

// frameNode is one link of an error's stack; next is the frame one call
// closer to where the error arose.
type frameNode struct {
	Frame
	next *frameNode
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

func (e *Error) TypeName() string { return "Error" }

func (e *Error) String() string {
	return e.Value
}

func (e *Error) ToString() (string, error) { return e.String(), nil }

func (e *Error) ToBool() (bool, error) { return true, nil }

func (e *Error) Equals(other Object) (bool, error) {
	v, ok := other.(*Error)
	return ok && e == v, nil
}

// Hash follows identity, so errors and error kinds can be dict keys.
func (e *Error) Hash() (uint64, error) { return identityHash(e), nil }

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
		return &Error{Value: e.Value, Wrapped: e, frames: &frameNode{Frame: frame, next: e.frames}}
	}
	return &Error{Value: err.Error(), Wrapped: err, frames: &frameNode{Frame: frame}}
}

// Traceback renders the language-level stack without exposing generated Go
// functions or the interpreter's own implementation stack. A frame repeated
// back to back, as in a deep recursion, is printed a few times and then
// summarised, and a very long stack keeps only its two ends.
func (e *Error) Traceback() string {
	if e.frames == nil {
		return e.Value
	}
	var lines []string
	var last string
	repeats := 0
	flush := func() {
		if repeats > tracebackRepeats {
			lines = append(lines, fmt.Sprintf("  [previous frame repeated %d more times]", repeats-tracebackRepeats))
		}
		repeats = 0
	}
	for f := e.frames; f != nil; f = f.next {
		line := f.render()
		if line == last {
			repeats++
			if repeats <= tracebackRepeats {
				lines = append(lines, line)
			}
			continue
		}
		flush()
		lines = append(lines, line)
		last = line
	}
	flush()
	if len(lines) > 2*tracebackEnds {
		omitted := len(lines) - 2*tracebackEnds
		tail := lines[len(lines)-tracebackEnds:]
		lines = append(lines[:tracebackEnds:tracebackEnds], fmt.Sprintf("  [%d more frames]", omitted))
		lines = append(lines, tail...)
	}
	var b strings.Builder
	b.WriteString("Traceback (most recent call last):\n")
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(e.Value)
	return b.String()
}

const (
	// tracebackRepeats is how many copies of a repeated frame are printed
	// before the rest are counted instead.
	tracebackRepeats = 3
	// tracebackEnds is how many lines are kept at each end of a traceback
	// that is still long after repeats are folded.
	tracebackEnds = 50
)

func (f *frameNode) render() string {
	var b strings.Builder
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
		return &Function{Name: "is", Fn: e.IsValue}, nil
	case "constructor":
		return ErrorConstructorFn, nil
	case "traceback":
		return String(e.Traceback()), nil
	}
	return nil, NewAttributeError("Error has no attribute '%s'", name)
}

func (e *Error) Attributes() []string {
	return []string{"attributes", "message", "wrap", "unwrap", "is", "constructor", "traceback"}
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
	if err := RequireNoArgs("unwrap", args); err != nil {
		return nil, err
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

// IsValue reports whether target appears anywhere in the receiver's cause chain,
// delegating to the standard library's errors.Is. Usage: err.is(target).
//
// The Go method deliberately avoids the name Is: methods named Is on error
// types are reserved for the standard errors.Is protocol and must have the
// signature Is(error) bool.
func (e *Error) IsValue(args CallArgs) (Object, error) {
	ap := NewArgParser("is", args)
	target := ap.Any("target")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	t, ok := target.(*Error)
	if !ok {
		return nil, NewTypeError("is() argument must be an Error, got %s", target.TypeName())
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
	OverflowError     = NewSentinelError("OverflowError", ArithmeticError)

	NotExistError   = NewSentinelError("NotExistError", IOError)
	ExistError      = NewSentinelError("ExistError", IOError)
	PermissionError = NewSentinelError("PermissionError", IOError)
	TimeoutError    = NewSentinelError("TimeoutError", IOError)
	NetworkError    = NewSentinelError("NetworkError", IOError)

	NotImplementedError = NewSentinelError("NotImplementedError", BaseError)
	RecursionError      = NewSentinelError("RecursionError", BaseError)
	InternalError       = NewSentinelError("InternalError", BaseError)
)

// NewInternalError creates an InternalError-tagged error. It marks failures of
// the runtime itself (recovered Go panics) rather than of the user's program.
func NewInternalError(format string, a ...any) *Error {
	return typedError(InternalError, format, a...)
}

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

// NewTypeErrorText tags a ready-made message as a TypeError. It skips the
// fmt pass, which matters on the argument-parsing path: passing a value
// through ...any boxes it, and that box is enough to make the caller's whole
// argument list escape to the heap.
func NewTypeErrorText(message string) *Error {
	return &Error{Value: message, Wrapped: TypeError}
}
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
func NewRecursionError(format string, a ...any) *Error {
	return typedError(RecursionError, format, a...)
}
func NewOverflowError(format string, a ...any) *Error {
	return typedError(OverflowError, format, a...)
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
func NewTimeoutError(format string, a ...any) *Error {
	return typedError(TimeoutError, format, a...)
}

// networkError has the method set of net.Error. Matching it structurally
// classifies exactly the errors net.Error would without linking the net
// package into every compiled program.
type networkError interface {
	error
	Timeout() bool
	Temporary() bool
}

// ErrorKind maps a native Go error to the Goblin error kind it represents,
// or fallback when it is none of the recognized kinds.
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
	var netErr networkError
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return TimeoutError
		}
		return NetworkError
	}
	return fallback
}

// WrapNativeError wraps a native Go error as a Goblin error of its kind.
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
	return NewError(inspect(message)), nil
}

// Raise validates the value thrown by a `raise` statement. Only Error values
// may be raised; raising anything else is itself an error. It returns the
// error to propagate through the Go error channel that both backends use.
func Raise(v Object) error {
	if e, ok := v.(*Error); ok {
		return e
	}
	return NewTypeError("raise expects an Error, got: %s", inspect(v))
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

type errorName struct{}

func (errorName) typeName() string { return "Error" }
