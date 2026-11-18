package object

import (
	"errors"
	"fmt"
	"testing"
)

// fakeExit replaces exitProcess for one test and returns the codes it was
// called with.
func fakeExit(t *testing.T) *[]int {
	t.Helper()
	var codes []int
	old := exitProcess
	exitProcess = func(code int) { codes = append(codes, code) }
	t.Cleanup(func() { exitProcess = old })
	return &codes
}

func TestExitCode(t *testing.T) {
	if _, ok := ExitCode(nil); ok {
		t.Error("ExitCode(nil) reports an exit")
	}
	ordinary := NewValueError("ordinary")
	if IsExit(ordinary) {
		t.Error("an ordinary error is an exit")
	}
	req := NewExit(3)
	if code, ok := ExitCode(req); !ok || code != 3 {
		t.Errorf("ExitCode(NewExit(3)) = %d, %v", code, ok)
	}
	if IsExit(ordinary) {
		t.Error("an ordinary error is an exit once an exit was requested")
	}
	if _, ok := ExitCode(nil); ok {
		t.Error("ExitCode(nil) reports an exit once an exit was requested")
	}
	for name, err := range map[string]error{
		"WithFrame":       WithFrame(req, Frame{Module: "m", Function: "f", Line: 1}),
		"WrapError":       WrapError(IOError, "wrapped", req),
		"NewWrappedError": NewWrappedError("wrapped", req),
		"fmt.Errorf":      fmt.Errorf("wrapped: %w", req),
	} {
		if code, ok := ExitCode(err); !ok || code != 3 {
			t.Errorf("%s: ExitCode = %d, %v", name, code, ok)
		}
	}
}

func TestExitMatchesNoKind(t *testing.T) {
	req := NewExit(2)
	for _, kind := range []*Error{BaseError, ValueError, IOError} {
		matched, err := ErrorMatches(ErrorValue(req), kind)
		if err != nil || matched {
			t.Errorf("ErrorMatches(exit, %s) = %v, %v", kind.Value, matched, err)
		}
		if errors.Is(req, kind) {
			t.Errorf("errors.Is(exit, %s)", kind.Value)
		}
	}
}

func TestExitNow(t *testing.T) {
	codes := fakeExit(t)
	ExitNow(nil)
	ExitNow(NewValueError("ordinary"))
	if len(*codes) != 0 {
		t.Fatalf("ExitNow ended the process for %v", *codes)
	}
	ExitNow(WithFrame(NewExit(7), Frame{Module: "m"}))
	if len(*codes) != 1 || (*codes)[0] != 7 {
		t.Fatalf("exit codes = %v, want [7]", *codes)
	}
}

// The base kind matches every error but the exit request, also once the
// request has collected stack frames.
func TestExitDoesNotMatchTheBaseKind(t *testing.T) {
	req := WithFrame(WithFrame(NewExit(4), Frame{Function: "f"}), Frame{Function: "g"})
	for _, caught := range []Object{ErrorValue(req), ErrorValue(NewExit(4))} {
		if matched, err := ErrorMatches(caught, BaseError); err != nil || matched {
			t.Errorf("ErrorMatches(exit, Error) = %v, %v", matched, err)
		}
		if v, _ := caught.(*Error).IsValue(CallArgs{Positional: Args{BaseError}}); v != False {
			t.Errorf("exit.is(Error) = %v", v)
		}
	}
	if errors.Is(req, BaseError) {
		t.Error("errors.Is(exit, Error)")
	}
}
