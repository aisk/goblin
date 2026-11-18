package object

import (
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"
)

// Goblin is a handle to a function running in its own goroutine, obtained
// from spawn, which starts the function immediately. wait() joins it and
// delivers its return value or re-raises its error. The result fields are
// written exactly once before done is closed, and only read after done is
// observed closed, so the channel provides the necessary happens-before edge
// without a lock.
type Goblin struct {
	OpaqueBase
	done   chan struct{}
	result Object
	err    error
}

var _ Object = &Goblin{}

func (g *Goblin) String() string {
	if g.finished() {
		return "<goblin done>"
	}
	return "<goblin running>"
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
	case "done":
		return Bool(g.finished()), nil
	case "wait":
		return &Function{Name: "wait", Fn: g.Wait}, nil
	default:
		return nil, NewAttributeError("Goblin has no attribute '%s'", name)
	}
}

func (g *Goblin) Attributes() []string {
	return []string{"done", "wait"}
}

// finished reports without blocking whether the function has returned; it
// backs the done property.
func (g *Goblin) finished() bool {
	select {
	case <-g.done:
		return true
	default:
		return false
	}
}

// Wait blocks until the function finishes, then returns its result or
// re-raises its error. The outcome is cached, so wait() may be called any
// number of times (and from several goblins at once) with the same answer.
// An optional timeout in seconds raises TimeoutError if the function is
// still running when it expires; a nil timeout means wait forever. A goblin
// that has already finished is always delivered, even with timeout=0.
func (g *Goblin) Wait(args CallArgs) (Object, error) {
	p := NewArgParser("wait", args)
	timeout, hasTimeout := p.Timeout("timeout")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if !g.finished() {
		if hasTimeout {
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			select {
			case <-g.done:
			case <-timer.C:
				return nil, NewTimeoutError("wait() timed out after %g seconds", timeout.Seconds())
			}
		} else {
			<-g.done
		}
	}
	if g.err != nil {
		unwaited.observe(g)
		return nil, g.err
	}
	if g.result == nil {
		return Nil, nil
	}
	return g.result, nil
}

var SpawnFn = &Function{Name: "spawn", Fn: Spawn}

// Spawn implements spawn(fn, *args, **kwargs): it starts fn in a new
// goroutine with the remaining arguments, positional and keyword alike, and
// returns the Goblin handle. fn is positional-only, so every keyword argument
// reaches fn. A Go-level panic inside the function is contained and surfaces
// as an InternalError from wait(), matching how the top-level runtime reports
// panics, instead of killing the process. An error nobody waits for is
// reported by ReportUnwaitedErrors. A goblin that calls os.exit unwinds its
// own deferred calls and then ends the process from its goroutine; the
// other goblins, the main one included, are stopped where they are.
func Spawn(args CallArgs) (Object, error) {
	// fn cannot be passed by keyword, so the messages do not name it.
	if len(args.Positional) == 0 {
		return nil, NewTypeError("spawn() takes the function to run as its first positional argument")
	}
	fn, ok := AsFunction(args.Positional[0])
	if !ok {
		return nil, NewTypeError("spawn() first argument must be callable, got %s", args.Positional[0].TypeName())
	}
	// The arguments only live for this call (see CallArgs); the goroutine
	// runs after it returns, so it gets its own copy.
	call := CallArgs{Positional: append(Args(nil), args.Positional[1:]...)}
	if len(args.Keyword) > 0 {
		call.Keyword = append(Kwargs(nil), args.Keyword...)
	}
	g := &Goblin{OpaqueBase: MakeOpaqueBase("Goblin"), done: make(chan struct{})}
	go func() {
		defer close(g.done)
		defer func() {
			if r := recover(); r != nil {
				g.err = NewInternalError("panic in goblin: %v", r)
			}
			if g.err != nil {
				unwaited.add(g)
			}
		}()
		g.result, g.err = fn.Call(call)
		// A goblin that called os.exit has run its deferred calls; the
		// process ends here, before the request could be stored as a
		// failure, reported or delivered by wait().
		if g.err != nil {
			ExitNow(g.err)
		}
	}()
	return g, nil
}

var WaitAnyFn = &Function{Name: "wait_any", Fn: WaitAny}

// WaitAny implements wait_any(goblins, timeout=nil): it returns the first of
// the goblins to finish, without taking its result or raising its error. When
// several have already finished it returns the earliest in the list, so the
// answer is reproducible; only when none has does it block, and the order of
// goblins finishing together while it blocks is unspecified.
func WaitAny(args CallArgs) (Object, error) {
	p := NewArgParser("wait_any", args)
	iterable := p.Any("goblins")
	timeout, hasTimeout := p.Timeout("timeout")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	items, err := Collect(iterable)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, NewValueError("wait_any() needs at least one goblin")
	}
	goblins := make([]*Goblin, len(items))
	for i, item := range items {
		g, ok := item.(*Goblin)
		if !ok {
			return nil, NewTypeError("wait_any() argument 'goblins' must contain only Goblin values, got %s", item.TypeName())
		}
		goblins[i] = g
	}
	for _, g := range goblins {
		if g.finished() {
			return g, nil
		}
	}
	cases := make([]reflect.SelectCase, len(goblins), len(goblins)+1)
	for i, g := range goblins {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(g.done)}
	}
	if hasTimeout {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(timer.C)})
	}
	chosen, _, _ := reflect.Select(cases)
	if chosen == len(goblins) {
		return nil, NewTimeoutError("wait_any() timed out after %g seconds", timeout.Seconds())
	}
	return goblins[chosen], nil
}

// unwaited holds the goblins that failed and whose error no wait() has
// observed yet, in the order they finished.
var unwaited unwaitedSet

type unwaitedSet struct {
	mu      sync.Mutex
	goblins []*Goblin
}

func (s *unwaitedSet) add(g *Goblin) {
	s.mu.Lock()
	s.goblins = append(s.goblins, g)
	s.mu.Unlock()
}

func (s *unwaitedSet) observe(g *Goblin) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, pending := range s.goblins {
		if pending == g {
			s.goblins = append(s.goblins[:i], s.goblins[i+1:]...)
			return
		}
	}
}

// ReportUnwaitedErrors writes one line per goblin that failed without any
// wait() observing its error, in the order they finished, and forgets them.
// Both backends call it when a program ends normally, and the REPL after each
// evaluation, so an error in a goblin nobody waits for is never silently
// lost. A goblin still running is not reported.
func ReportUnwaitedErrors(w io.Writer) {
	unwaited.mu.Lock()
	goblins := unwaited.goblins
	unwaited.goblins = nil
	unwaited.mu.Unlock()
	for _, g := range goblins {
		fmt.Fprintf(w, "uncaught error in goblin: %v\n", g.err)
	}
}
