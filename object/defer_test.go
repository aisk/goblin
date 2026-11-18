package object

import (
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestDefersRunLastInFirstOut(t *testing.T) {
	var order []int
	var d Defers
	for i := 0; i < 3; i++ {
		d.Push(func() error {
			order = append(order, i)
			return nil
		})
	}
	if err := d.Run(nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reflect.DeepEqual(order, []int{2, 1, 0}) {
		t.Errorf("order = %v, want [2 1 0]", order)
	}
}

func TestDefersRunKeepsTheErrorInFlight(t *testing.T) {
	inFlight := NewError("in flight")
	var d Defers
	d.Push(func() error { return nil })
	if err := d.Run(inFlight); !errors.Is(err, inFlight) {
		t.Errorf("Run = %v, want the error in flight", err)
	}
	if err := (Defers)(nil).Run(inFlight); !errors.Is(err, inFlight) {
		t.Errorf("empty Run = %v, want the error in flight", err)
	}
}

func TestDefersRunReplacesTheErrorAndKeepsGoing(t *testing.T) {
	first, second := NewError("first"), NewError("second")
	ran := 0
	var d Defers
	d.Push(func() error { ran++; return second })
	d.Push(func() error { ran++; return nil })
	d.Push(func() error { ran++; return first })
	err := d.Run(NewError("in flight"))
	if ran != 3 {
		t.Errorf("ran %d deferred calls, want 3", ran)
	}
	// The call that runs last decides.
	if !errors.Is(err, second) {
		t.Errorf("Run = %v, want the last error raised", err)
	}
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	defer func() { os.Stderr = old }()
	fn()
	os.Stderr = old
	w.Close()
	return <-done
}

// An ordinary error raised while an exit is in flight is reported and does
// not replace the exit; the remaining calls run and the code is kept.
func TestDefersRunReportsAnErrorDuringExit(t *testing.T) {
	ran := 0
	var d Defers
	d.Push(func() error { ran++; return nil })
	d.Push(func() error { ran++; return NewValueError("cannot clean up") })
	d.Push(func() error { ran++; return nil })
	var err error
	out := captureStderr(t, func() { err = d.Run(NewExit(4)) })
	if ran != 3 {
		t.Errorf("ran %d deferred calls, want 3", ran)
	}
	if code, ok := ExitCode(err); !ok || code != 4 {
		t.Errorf("Run = %v, want exit 4", err)
	}
	if out != "uncaught error in deferred call: cannot clean up\n" {
		t.Errorf("stderr = %q", out)
	}
}

// An exit with status 0 becomes status 1 once a deferred call failed.
func TestDefersRunTurnsExitZeroIntoOne(t *testing.T) {
	var d Defers
	d.Push(func() error { return NewValueError("first failure") })
	d.Push(func() error { return NewValueError("second failure") })
	var err error
	out := captureStderr(t, func() { err = d.Run(NewExit(0)) })
	if code, ok := ExitCode(err); !ok || code != 1 {
		t.Errorf("Run = %v, want exit 1", err)
	}
	want := "uncaught error in deferred call: second failure\nuncaught error in deferred call: first failure\n"
	if out != want {
		t.Errorf("stderr = %q", out)
	}
}

// A deferred call that exits replaces an exit in flight; the last one to
// run decides the code.
func TestDefersRunExitReplacesExit(t *testing.T) {
	var d Defers
	d.Push(func() error { return NewExit(7) })
	d.Push(func() error { return NewExit(6) })
	if code, ok := ExitCode(d.Run(NewExit(5))); !ok || code != 7 {
		t.Errorf("exit code %d, %v, want 7", code, ok)
	}
}

// A deferred call that exits replaces an ordinary error in flight, and
// starts an exit on a normal return, printing nothing.
func TestDefersRunExitReplacesAnError(t *testing.T) {
	var d Defers
	d.Push(func() error { return NewExit(8) })
	var err error
	out := captureStderr(t, func() { err = d.Run(NewError("lost")) })
	if code, ok := ExitCode(err); !ok || code != 8 {
		t.Errorf("Run = %v, want exit 8", err)
	}
	if code, ok := ExitCode(d.Run(nil)); !ok || code != 8 {
		t.Errorf("exit code %d, %v on a normal return, want 8", code, ok)
	}
	if out != "" {
		t.Errorf("stderr = %q", out)
	}
}

// A deferred call that handles its own errors leaves the exit in flight.
func TestDefersRunKeepsAnExitInFlight(t *testing.T) {
	var d Defers
	d.Push(func() error { return nil })
	if code, ok := ExitCode(d.Run(NewExit(9))); !ok || code != 9 {
		t.Errorf("exit code %d, %v, want 9", code, ok)
	}
}
