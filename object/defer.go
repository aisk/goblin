package object

import (
	"fmt"
	"os"
)

// Defers is the stack of calls one activation (a function call or a module
// body) has deferred. Both backends push a closure per `defer` statement,
// holding the callee and arguments it evaluated, and run the stack when the
// activation ends.
type Defers []func() error

// Push adds a deferred call.
func (d *Defers) Push(call func() error) { *d = append(*d, call) }

// Run runs the deferred calls, last pushed first, for an activation ending
// with err (nil for a normal return), and returns the error it then ends
// with. Every call runs even when an earlier one raises; an error a
// deferred call raises replaces the one in flight, and so does an exit
// request (os.exit) a deferred call raises.
//
// While an exit request is in flight an ordinary error does not replace it:
// otherwise an outer catch would receive that error and the program would
// carry on after os.exit. The error is written to stderr as one line, the
// remaining calls still run, and a requested status of 0 becomes 1, since a
// process that printed an uncaught error does not report success.
func (d Defers) Run(err error) error {
	for i := len(d) - 1; i >= 0; i-- {
		raised := d[i]()
		if raised == nil {
			continue
		}
		if code, exiting := ExitCode(err); exiting && !IsExit(raised) {
			fmt.Fprintf(os.Stderr, "uncaught error in deferred call: %v\n", raised)
			if code == 0 {
				err = NewExit(1)
			}
			continue
		}
		err = raised
	}
	return err
}
