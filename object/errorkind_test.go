package object

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"testing"
)

// ErrorKind matches net.Error structurally so the runtime does not link the
// net package; this pins that the classification is unchanged.
func TestErrorKindClassifiesNativeErrors(t *testing.T) {
	_, notExist := os.Open("/definitely/not/here")
	_, refused := net.Dial("tcp", "127.0.0.1:1")
	timeout := &net.DNSError{Err: "timeout", IsTimeout: true}
	cases := []struct {
		name string
		err  error
		want *Error
	}{
		{"not exist", notExist, NotExistError},
		{"exist", fmt.Errorf("wrapped: %w", fs.ErrExist), ExistError},
		{"permission", fs.ErrPermission, PermissionError},
		{"network", refused, NetworkError},
		{"network timeout", timeout, TimeoutError},
		{"deadline", context.DeadlineExceeded, TimeoutError},
		{"other", fmt.Errorf("boom"), IOError},
	}
	for _, c := range cases {
		if c.err == nil {
			t.Fatalf("%s: expected an error to classify", c.name)
		}
		if got := ErrorKind(c.err, IOError); got != c.want {
			t.Errorf("%s: ErrorKind(%v) = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

// Calling an error makes an error of that kind: the message stands alone,
// the cause is the kind, and keyword arguments become data.
func TestCallingAnErrorMakesOneOfItsKind(t *testing.T) {
	v, err := Call(ValueError, CallArgs{
		Positional: Args{String("invalid port: abc")},
		Keyword:    Kwargs{{Name: "port", Value: String("abc")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := v.(*Error)
	if e.Value != "invalid port: abc" || e.Cause() != ValueError {
		t.Fatalf("got %q with cause %v", e.Value, e.Cause())
	}
	if ok, _ := ErrorMatches(e, ValueError); !ok {
		t.Error("an error made from ValueError does not match it")
	}
	if ok, _ := ErrorMatches(e, BaseError); !ok {
		t.Error("an error made from ValueError does not match its parent kind")
	}
	if port, ok := e.Field("port"); !ok || port != String("abc") {
		t.Errorf("port = %v, %v", port, ok)
	}

	// Without a message the kind's own is used.
	v, err = Call(KeyError, CallArgs{})
	if err != nil || v.(*Error).Value != "KeyError" {
		t.Fatalf("KeyError() = %v, %v", v, err)
	}

	// The built-in members cannot be data names.
	for _, name := range []string{"traceback", "cause", "wrap", "is"} {
		_, err := Call(ValueError, CallArgs{Keyword: Kwargs{{Name: name, Value: Integer(1)}}})
		want := "Error() cannot use '" + name + "' as a data name"
		if err == nil || err.Error() != want {
			t.Errorf("%s: error %v, want %q", name, err, want)
		}
		if ok, _ := ErrorMatches(ErrorValue(err), ValueError); !ok {
			t.Errorf("%s: the error is not a ValueError", name)
		}
	}
	_, err = Call(BaseError, CallArgs{Keyword: Kwargs{{Name: "cause", Value: Integer(1)}}})
	if err == nil || err.Error() != "Error() cannot use 'cause' as a data name" {
		t.Errorf("Error(cause=1): %v", err)
	}
}

// Data is read through the stack-frame wrappers a raised error collects,
// through wraps, from the kind an error was made from, and from a native
// error's kind; the nearest value wins.
func TestErrorFieldFollowsTheCauseChain(t *testing.T) {
	kind := NewErrorKind("Retryable", BaseError).With("retry", True).With("attempts", Integer(3))
	made, err := kind.New(CallArgs{Positional: Args{String("timed out")}, Keyword: Kwargs{{Name: "attempts", Value: Integer(5)}}})
	if err != nil {
		t.Fatal(err)
	}
	raised := WithFrame(WithFrame(made.(*Error), Frame{Function: "inner"}), Frame{Function: "outer"}).(*Error)
	wrapped := NewWrappedError("loading", raised)
	native := WrapError(kind, "reading", fmt.Errorf("boom"))

	cases := []struct {
		e     *Error
		name  string
		value Object
	}{
		{raised, "attempts", Integer(5)},
		{raised, "retry", True},
		{wrapped, "attempts", Integer(5)},
		{wrapped, "retry", True},
		{native, "retry", True},
		{kind, "attempts", Integer(3)},
	}
	for _, c := range cases {
		got, err := c.e.GetAttr(c.name)
		if err != nil || got != c.value {
			t.Errorf("%q.%s = %v, %v; want %v", c.e.Value, c.name, got, err, c.value)
		}
	}
	if _, ok := kind.Field("missing"); ok {
		t.Error("a missing name was found")
	}
	if _, err := wrapped.GetAttr("missing"); err == nil || err.Error() != "Error has no attribute 'missing'" {
		t.Errorf("missing attribute: %v", err)
	}
	want := []string{"message", "traceback", "cause", "wrap", "is", "attempts", "retry"}
	if got := wrapped.Attributes(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Attributes() = %v, want %v", got, want)
	}
}

// ErrorMatches is what `catch e: kind` runs: a kind that is not an Error is
// a TypeError, and anything caught that is not an Error matches nothing.
func TestErrorMatches(t *testing.T) {
	caught := WithFrame(NewKindError(IndexError, "list index out of range"), Frame{Function: "f"}).(*Error)
	for _, c := range []struct {
		kind *Error
		want bool
	}{{IndexError, true}, {LookupError, true}, {KeyError, false}, {BaseError, true}} {
		if got, err := ErrorMatches(caught, c.kind); err != nil || got != c.want {
			t.Errorf("matches %s = %v, %v; want %v", c.kind.Value, got, err, c.want)
		}
	}
	if _, err := ErrorMatches(caught, Integer(1)); err == nil || err.Error() != "catch expects an Error to match, got Int" {
		t.Errorf("non-Error kind: %v", err)
	}
	// The kind is identified through its own stack-frame wrappers too.
	kind := WithFrame(IndexError, Frame{Function: "g"})
	if got, _ := ErrorMatches(caught, kind.(*Error)); !got {
		t.Error("a caught kind does not match")
	}
	if got, _ := ErrorMatches(String("x"), IndexError); got {
		t.Error("a String matched")
	}
}

// The global Error is the base kind: calling it makes an error of that kind,
// and every error matches it, whatever its chain holds, except the exit
// request os.exit raises.
func TestBaseErrorKind(t *testing.T) {
	if name, err := BaseError.GetAttr("name"); err != nil || name != String("Error") {
		t.Errorf("Error.name = %v, %v", name, err)
	}
	if _, err := ValueError.GetAttr("name"); err == nil {
		t.Error("ValueError.name should be free for data")
	}
	v, err := Call(BaseError, CallArgs{Positional: Args{String("plain")}, Keyword: Kwargs{{Name: "code", Value: Integer(2)}}})
	if err != nil {
		t.Fatal(err)
	}
	plain := v.(*Error)
	if plain.Value != "plain" || plain.Cause() != BaseError {
		t.Fatalf("Error(\"plain\") = %q with cause %v", plain.Value, plain.Cause())
	}
	if code, ok := plain.Field("code"); !ok || code != Integer(2) {
		t.Errorf("code = %v, %v", code, ok)
	}
	if v, err := Call(BaseError, CallArgs{}); err != nil || v.(*Error).Value != "Error" {
		t.Fatalf("Error() = %v, %v", v, err)
	}
	if BaseError.Cause() != Nil || ValueError.Cause() != BaseError {
		t.Fatal("the base kind has no cause and is the parent of the root kinds")
	}

	sentinel := plain
	framed := func(err error) Object {
		return ErrorValue(WithFrame(WithFrame(err, Frame{Function: "f"}), Frame{Function: "g"}))
	}
	for name, e := range map[string]Object{
		"kind error":        NewValueError("bad"),
		"plain":             plain,
		"sentinel raised":   framed(sentinel),
		"wrapped":           NewWrappedError("loading", NewIndexError("out of range")),
		"internal":          NewInternalError("panic"),
		"native kind":       WrapNativeError(IOError, "read", fs.ErrNotExist),
		"framed native":     framed(fmt.Errorf("no kind")),
		"native":            ErrorValue(fmt.Errorf("no kind")),
		"kindless value":    &Error{Value: "built by hand"},
		"wrapping a native": &Error{Value: "x", Wrapped: fmt.Errorf("no kind")},
		"a kind":            KeyError,
		"the base kind":     BaseError,
	} {
		if ok, err := ErrorMatches(e, BaseError); !ok || err != nil {
			t.Errorf("%s: ErrorMatches(e, Error) = %v, %v", name, ok, err)
		}
		if ok, err := e.(*Error).IsValue(CallArgs{Positional: Args{BaseError}}); ok != True || err != nil {
			t.Errorf("%s: e.is(Error) = %v, %v", name, ok, err)
		}
	}
	// A sentinel made with Error(...) still matches only itself among
	// errors made from the base kind.
	other, _ := Call(BaseError, CallArgs{Positional: Args{String("plain")}})
	if ok, _ := ErrorMatches(framed(sentinel), sentinel); !ok {
		t.Error("a raised sentinel does not match itself")
	}
	if ok, _ := ErrorMatches(other, sentinel); ok {
		t.Error("another error matches the sentinel")
	}
	if eq, _ := framed(sentinel).Equals(sentinel); !eq {
		t.Error("a raised sentinel does not equal itself")
	}
	if ok, _ := ErrorMatches(Integer(1), BaseError); ok {
		t.Error("a value that is not an error matches the base kind")
	}
	if TypeOf(plain) != BaseError || TypeOf(BaseError) != BaseError {
		t.Error("the type of an error is the base kind")
	}
}
