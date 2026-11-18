package object

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
)

// exitRequest is the cause of the error os.exit raises. It is not an Error
// kind and is not reachable from Goblin code, so no kind matches it.
type exitRequest struct{ code int }

func (r *exitRequest) Error() string { return fmt.Sprintf("exit status %d", r.code) }

// exitRequested is set by the first NewExit of the process and never
// cleared. Until then ExitCode and IsExit answer after one atomic load, so a
// program that never calls os.exit pays nothing more for them.
var exitRequested atomic.Bool

// exitProcess ends the process; tests in this package replace it.
var exitProcess = os.Exit

// NewExit returns the error that makes the calling task unwind and the
// process end with code. The calling task runs its pending deferred calls
// on the way out, as for any error, and the process ends at the top of the
// task (main.go, the generated main, Spawn). It is a Goblin error so that
// library code passing Goblin errors through unchanged passes it too, but
// its chain holds no error kind and the catch implementations of both
// backends test IsExit first, so no catch clause receives it.
func NewExit(code int) error {
	exitRequested.Store(true)
	return &Error{Value: "exit", Wrapped: &exitRequest{code: code}}
}

// ExitCode reports whether err is, or wraps, an exit request, and its code.
func ExitCode(err error) (int, bool) {
	if err == nil || !exitRequested.Load() {
		return 0, false
	}
	var r *exitRequest
	if errors.As(err, &r) {
		return r.code, true
	}
	return 0, false
}

// IsExit reports whether err is, or wraps, an exit request.
func IsExit(err error) bool {
	_, ok := ExitCode(err)
	return ok
}

// ExitNow ends the process with the code of the exit request err carries.
// It is for the top of a task and for the places an exit request cannot
// propagate from; it returns without doing anything when err is not one.
func ExitNow(err error) {
	if code, ok := ExitCode(err); ok {
		exitProcess(code)
	}
}
