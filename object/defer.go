package object

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
// deferred call raises replaces the one in flight.
func (d Defers) Run(err error) error {
	for i := len(d) - 1; i >= 0; i-- {
		if raised := d[i](); raised != nil {
			err = raised
		}
	}
	return err
}
