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
	// data is the named data the error was made with (`K(msg, code=1)`),
	// read as attributes; see Field. There are rarely more than a few
	// entries, so a slice beats a map.
	data []errorField
}

// errorField is one named piece of data carried by an error.
type errorField struct {
	name  string
	value Object
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

// NewErrorKind makes an error kind: an error named name whose cause is
// parent, so errors of the kind match parent too. A module exports its own
// kinds as module values (exec.ExitError).
func NewErrorKind(name string, parent *Error) *Error {
	return &Error{Value: name, Wrapped: parent}
}

// NewKindError builds an error of kind with a formatted message: the message
// stands alone and the cause is kind, so the error matches kind (and its
// parents) with is() and `catch e: kind`. It is what calling a kind does in
// Goblin, and how the runtime reports its own failures.
func NewKindError(kind *Error, format string, a ...any) *Error {
	return &Error{Value: fmt.Sprintf(format, a...), Wrapped: kind}
}

// With attaches data to a freshly built error and returns it, for chaining:
// object.WrapNativeError(object.IOError, msg, err).With("path", p). It must
// not be used on an error that may already have been raised or shared.
func (e *Error) With(name string, value Object) *Error {
	for i := range e.data {
		if e.data[i].name == name {
			e.data[i].value = value
			return e
		}
	}
	e.data = append(e.data, errorField{name: name, value: value})
	return e
}

// Field looks name up in the error's data and then down its cause chain:
// what it wraps, or the kind of a native error. The first hit wins, so data
// stays readable through a wrap and data given to a kind is seen on every
// error made from it. The stack-frame wrappers a propagating error collects
// carry no data and are passed through.
func (e *Error) Field(name string) (Object, bool) {
	var next error = e.origin()
	for next != nil {
		switch cur := next.(type) {
		case *Error:
			for i := range cur.data {
				if cur.data[i].name == name {
					return cur.data[i].value, true
				}
			}
			next = cur.Wrapped
		case typedCause:
			next = cur.base
		default:
			return nil, false
		}
	}
	return nil, false
}

// fieldNames lists the data names visible through the chain, nearest first,
// without duplicates.
func (e *Error) fieldNames() []string {
	var names []string
	seen := map[string]bool{}
	var next error = e.origin()
	for next != nil {
		switch cur := next.(type) {
		case *Error:
			for _, f := range cur.data {
				if !seen[f.name] {
					seen[f.name] = true
					names = append(names, f.name)
				}
			}
			next = cur.Wrapped
		case typedCause:
			next = cur.base
		default:
			next = nil
		}
	}
	return names
}

// NewWrappedError builds an error carrying message that wraps cause, following
// Go's convention where the rendered message is "message: cause".
func NewWrappedError(message string, cause error) *Error {
	return &Error{Value: message + ": " + cause.Error(), Wrapped: cause}
}

// WrapError wraps a native Go error as a Goblin error of kind base, adding
// message. It builds two layers, so that `cause` reaches a typed error: the
// outer one carries "message: cause", the inner one the native error's own
// message, matching both base and the native error under errors.Is.
func WrapError(base *Error, message string, cause error) *Error {
	inner := &Error{Value: cause.Error(), Wrapped: typedCause{cause: cause, base: base}}
	return &Error{Value: message + ": " + inner.Value, Wrapped: inner}
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

// origin is the error as raised: the receiver without the stack-frame
// wrappers WithFrame adds while it propagates. The error a `catch` binds is
// such a wrapper, so identity, hashing and the cause chain all go through
// origin; otherwise how deep the raise happened would change the answers.
func (e *Error) origin() *Error {
	for e.frames != nil {
		inner, ok := e.Wrapped.(*Error)
		if !ok {
			break
		}
		e = inner
	}
	return e
}

// Equals is identity of the raised error, so a caught error equals the
// sentinel or value that was raised.
func (e *Error) Equals(other Object) (bool, error) {
	v, ok := other.(*Error)
	return ok && e.origin() == v.origin(), nil
}

// Hash follows identity, so errors and error kinds can be dict keys.
func (e *Error) Hash() (uint64, error) { return identityHash(e.origin()), nil }

// Cause is the next error down the cause chain: what the error wraps, the
// kind of a native error (BaseError when it has no other), or the parent of
// an error kind. Only BaseError itself has no cause and answers Nil.
func (e *Error) Cause() Object {
	origin := e.origin()
	switch w := origin.Wrapped.(type) {
	case nil:
		return Nil
	case *Error:
		return w
	case typedCause:
		return w.base
	default:
		if origin.frames != nil {
			// A frame wrapper around a native error: the native error is
			// the error itself, not its cause, and has no kind but the
			// base one.
			return BaseError
		}
		return nativeError(w)
	}
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
	case "message":
		return String(e.Value), nil
	case "wrap":
		return &Function{Name: "wrap", Fn: e.Wrap}, nil
	case "cause":
		return e.Cause(), nil
	case "is":
		return &Function{Name: "is", Fn: e.IsValue}, nil
	case "traceback":
		return String(e.Traceback()), nil
	}
	if name == "name" && e == BaseError {
		return String(e.TypeName()), nil
	}
	if v, ok := e.Field(name); ok {
		return v, nil
	}
	return nil, NewAttributeError("Error has no attribute '%s'", name)
}

// Attributes lists the built-in members, then the data names visible through
// the cause chain, nearest first. The base kind is what type_of returns for
// an error, so it alone also answers `name`, as every other type does; on
// any other error `name` stays free for data.
func (e *Error) Attributes() []string {
	names := []string{"message", "traceback", "cause", "wrap", "is"}
	if e == BaseError {
		names = append(names, "name")
	}
	return append(names, e.fieldNames()...)
}

// reservedDataNames are the built-in members a data name would hide.
// `message` is the constructor's parameter, so it cannot be data.
func reservedDataName(name string) bool {
	switch name {
	case "traceback", "cause", "wrap", "is":
		return true
	}
	return false
}

// newError builds the error `kind(...)` makes: the message, rendered as
// print renders it, defaulting to def, and every remaining keyword argument
// as data.
func newError(kind *Error, def Object, args CallArgs) (*Error, error) {
	ap := NewArgParser("Error", args)
	message := ap.AnyOr("message", def)
	data := ap.KeywordRest()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	e := &Error{Value: inspect(message), Wrapped: kind}
	for _, kw := range data {
		if reservedDataName(kw.Name) {
			return nil, NewValueError("Error() cannot use '%s' as a data name", kw.Name)
		}
		e.data = append(e.data, errorField{name: kw.Name, value: kw.Value})
	}
	return e, nil
}

// New is what calling an error does: `K(message=K.message, **data)` makes a
// new error of kind K, whose message stands alone and whose cause is K.
func (e *Error) New(args CallArgs) (Object, error) {
	kind := e.origin()
	return newError(kind, String(kind.Value), args)
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
	if _, ok := target.(*Error); !ok {
		return nil, NewTypeError("is() argument must be an Error, got %s", target.TypeName())
	}
	matched, err := ErrorMatches(e, target)
	return Bool(matched), err
}

// ErrorMatches reports whether kind appears anywhere in the cause chain of
// caught, the error a catch clause is offered. It is what `catch e: kind`
// and e.is(kind) run; a kind that is not an Error is a TypeError. The base
// kind (the global `Error`) matches every error except the exit request, so
// `catch e: Error` handles exactly what a bare `catch e` handles.
func ErrorMatches(caught Object, kind Object) (bool, error) {
	k, ok := kind.(*Error)
	if !ok {
		return false, NewTypeError("catch expects an Error to match, got %s", kind.TypeName())
	}
	e, ok := caught.(*Error)
	if !ok {
		return false, nil
	}
	k = k.origin()
	if k == BaseError {
		// Every error is of the base kind, whatever its chain holds: a
		// native failure wrapped without a kind still matches. The exit
		// request is not an error a program handles, so it never does.
		return !IsExit(e), nil
	}
	return errors.Is(e, k), nil
}

var _ error = (*Error)(nil)

// Predefined error values covering the common failure kinds. Each is a distinct
// sentinel that can be raised, given context with .wrap(), matched with .is(),
// or used as the base for a derived error. BaseError is the global `Error`:
// the root kind, with no cause of its own, that every other kind descends
// from and every error matches.
var (
	BaseError       = NewError("Error")
	TypeError       = NewErrorKind("TypeError", BaseError)
	ValueError      = NewErrorKind("ValueError", BaseError)
	LookupError     = NewErrorKind("LookupError", BaseError)
	ArithmeticError = NewErrorKind("ArithmeticError", BaseError)
	IOError         = NewErrorKind("IOError", BaseError)

	ParseError        = NewErrorKind("ParseError", ValueError)
	IndexError        = NewErrorKind("IndexError", LookupError)
	KeyError          = NewErrorKind("KeyError", LookupError)
	AttributeError    = NewErrorKind("AttributeError", LookupError)
	NameError         = NewErrorKind("NameError", LookupError)
	ImportError       = NewErrorKind("ImportError", LookupError)
	ZeroDivisionError = NewErrorKind("ZeroDivisionError", ArithmeticError)
	OverflowError     = NewErrorKind("OverflowError", ArithmeticError)

	NotExistError   = NewErrorKind("NotExistError", IOError)
	ExistError      = NewErrorKind("ExistError", IOError)
	PermissionError = NewErrorKind("PermissionError", IOError)
	TimeoutError    = NewErrorKind("TimeoutError", IOError)
	NetworkError    = NewErrorKind("NetworkError", IOError)

	NotImplementedError = NewErrorKind("NotImplementedError", BaseError)
	RecursionError      = NewErrorKind("RecursionError", BaseError)
	InternalError       = NewErrorKind("InternalError", BaseError)
)

// NewInternalError creates an InternalError-tagged error. It marks failures of
// the runtime itself (recovered Go panics) rather than of the user's program.
func NewInternalError(format string, a ...any) *Error {
	return NewKindError(InternalError, format, a...)
}

// NewTypeError, NewValueError, NewIndexError, NewKeyError and
// NewZeroDivisionError create errors tagged with the matching sentinel. They are
// used throughout the runtime in place of fmt.Errorf so that raised failures can
// be caught by kind.
func NewTypeError(format string, a ...any) *Error { return NewKindError(TypeError, format, a...) }

// NewTypeErrorText tags a ready-made message as a TypeError. It skips the
// fmt pass, which matters on the argument-parsing path: passing a value
// through ...any boxes it, and that box is enough to make the caller's whole
// argument list escape to the heap.
func NewTypeErrorText(message string) *Error {
	return &Error{Value: message, Wrapped: TypeError}
}
func NewValueError(format string, a ...any) *Error {
	return NewKindError(ValueError, format, a...)
}
func NewIndexError(format string, a ...any) *Error {
	return NewKindError(IndexError, format, a...)
}
func NewKeyError(format string, a ...any) *Error { return NewKindError(KeyError, format, a...) }
func NewZeroDivisionError(format string, a ...any) *Error {
	return NewKindError(ZeroDivisionError, format, a...)
}
func NewRecursionError(format string, a ...any) *Error {
	return NewKindError(RecursionError, format, a...)
}
func NewOverflowError(format string, a ...any) *Error {
	return NewKindError(OverflowError, format, a...)
}
func NewAttributeError(format string, a ...any) *Error {
	return NewKindError(AttributeError, format, a...)
}
func NewNameError(format string, a ...any) *Error {
	return NewKindError(NameError, format, a...)
}
func NewImportError(format string, a ...any) *Error {
	return NewKindError(ImportError, format, a...)
}
func NewParseError(format string, a ...any) *Error {
	return NewKindError(ParseError, format, a...)
}
func NewTimeoutError(format string, a ...any) *Error {
	return NewKindError(TimeoutError, format, a...)
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
	return nativeError(err)
}

// nativeError presents a native Go error that carries no Goblin error as an
// Error of the base kind, still matching err under errors.Is.
func nativeError(err error) *Error {
	return &Error{Value: err.Error(), Wrapped: typedCause{cause: err, base: BaseError}}
}

type errorName struct{}

func (errorName) typeName() string { return "Error" }
