package object

import (
	"time"
)

// Goblin is a handle to a function running in its own goroutine. Construction
// starts the function immediately; wait() joins it and delivers its return
// value or re-raises its error, so unlike spawn nothing is silently dropped.
// The result fields are written exactly once before done is closed, and only
// read after done is observed closed, so the channel provides the necessary
// happens-before edge without a lock.
type Goblin struct {
	OpaqueBase
	done   chan struct{}
	result Object
	err    error
}

var _ Object = &Goblin{}

func (g *Goblin) String() string {
	select {
	case <-g.done:
		return "<goblin done>"
	default:
		return "<goblin running>"
	}
}

func (g *Goblin) ToString() (string, error) { return g.String(), nil }

func (g *Goblin) Equals(other Object) (bool, error) {
	v, ok := other.(*Goblin)
	return ok && g == v, nil
}

// Hash follows identity, so goblins can be dict keys.
func (g *Goblin) Hash() (uint64, error) { return identityHash(g), nil }

func (g *Goblin) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(g), nil
	case "done":
		return &Function{Name: "done", Fn: g.Done}, nil
	case "wait":
		return &Function{Name: "wait", Fn: g.Wait}, nil
	case "constructor":
		return GoblinConstructorFn, nil
	default:
		return nil, NewAttributeError("Goblin has no attribute '%s'", name)
	}
}

func (g *Goblin) Attributes() []string {
	return []string{"attributes", "done", "wait", "constructor"}
}

// Done reports without blocking whether the function has finished.
func (g *Goblin) Done(args CallArgs) (Object, error) {
	if err := RequireNoArgs("done", args); err != nil {
		return nil, err
	}
	select {
	case <-g.done:
		return True, nil
	default:
		return False, nil
	}
}

// Wait blocks until the function finishes, then returns its result or
// re-raises its error. The outcome is cached, so wait() may be called any
// number of times (and from several goblins at once) with the same answer.
// An optional timeout in seconds raises TimeoutError if the function is
// still running when it expires; a nil timeout means wait forever.
func (g *Goblin) Wait(args CallArgs) (Object, error) {
	p := NewArgParser("wait", args)
	tv, supplied := p.OptionalAny("timeout")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if supplied && tv != Nil {
		var seconds float64
		switch n := tv.(type) {
		case Integer:
			seconds = float64(n)
		case Float:
			seconds = float64(n)
		default:
			return nil, NewTypeError("wait() argument 'timeout' must be number or nil, got %s", tv.TypeName())
		}
		if seconds < 0 {
			return nil, NewValueError("wait() timeout must be non-negative, got %g", seconds)
		}
		timer := time.NewTimer(time.Duration(seconds * float64(time.Second)))
		defer timer.Stop()
		select {
		case <-g.done:
		case <-timer.C:
			return nil, NewTimeoutError("wait() timed out after %g seconds", seconds)
		}
	} else {
		<-g.done
	}
	if g.err != nil {
		return nil, g.err
	}
	if g.result == nil {
		return Nil, nil
	}
	return g.result, nil
}

var GoblinConstructorFn = &Function{Name: "Goblin", Fn: GoblinConstructor}

// GoblinConstructor starts fn in a new goroutine with the remaining positional
// arguments and returns a handle to it. A Go-level panic inside the function
// is contained and surfaces as an InternalError from wait(), matching how the
// top-level runtime reports panics, instead of killing the process.
func GoblinConstructor(args CallArgs) (Object, error) {
	p := NewArgParser("Goblin", args)
	fn := p.Func("fn")
	// The arguments only live for this call (see CallArgs); the goroutine
	// runs after it returns, so it gets its own copy.
	rest := append(Args(nil), p.Rest()...)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	g := &Goblin{OpaqueBase: MakeOpaqueBase("Goblin"), done: make(chan struct{})}
	go func() {
		defer close(g.done)
		defer func() {
			if r := recover(); r != nil {
				g.err = NewInternalError("panic in goblin: %v", r)
			}
		}()
		g.result, g.err = fn.Call(CallArgs{Positional: rest})
	}()
	return g, nil
}
