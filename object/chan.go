package object

import "time"

// Chan wraps a Go channel of Objects, exposing send/receive/close to Goblin.
// Element typing is dynamic: any Object can flow through the channel. A for
// loop over a Chan receives until the channel is closed and drained.
type Chan struct {
	OpaqueBase
	ch chan Object
}

var _ Object = &Chan{}
var _ Iterable = &Chan{}

// NewChan builds a Chan around a Go channel with the given buffer capacity.
func NewChan(capacity int) *Chan {
	return &Chan{OpaqueBase: MakeOpaqueBase("Chan"), ch: make(chan Object, capacity)}
}

func (c *Chan) String() string { return "<chan>" }

func (c *Chan) ToString() (string, error) { return c.String(), nil }

func (c *Chan) Equals(other Object) (bool, error) {
	v, ok := other.(*Chan)
	return ok && c == v, nil
}

// Hash follows identity, so channels can be dict keys.
func (c *Chan) Hash() (uint64, error) { return identityHash(c), nil }

func (c *Chan) GetAttr(name string) (Object, error) {
	switch name {
	case "send":
		return &Function{Name: "send", Fn: c.Send}, nil
	case "receive":
		return &Function{Name: "receive", Fn: c.Receive}, nil
	case "close":
		return &Function{Name: "close", Fn: c.Close}, nil
	default:
		return nil, NewAttributeError("Chan has no attribute '%s'", name)
	}
}

func (c *Chan) Attributes() []string {
	return []string{"send", "receive", "close"}
}

// Iter receives every value until the channel is closed; see Iterator.
func (c *Chan) Iter() ([]Object, error) { return Collect(c) }

// Iterator makes `for v in ch` receive one value per pass of the loop until
// the channel is closed and drained, which ends the loop. Loops iterating the
// same channel at once each get different values.
func (c *Chan) Iterator() (Iterator, error) {
	return IteratorFunc(func() (Object, bool, error) {
		v, ok := <-c.ch
		return v, ok, nil
	}), nil
}

// Send blocks until the value is delivered or buffered, or until the
// optional timeout in seconds expires, which raises TimeoutError. A send
// that can complete at once always does, even with timeout=0. Sending on a
// closed channel returns an error instead of panicking.
func (c *Chan) Send(args CallArgs) (_ Object, err error) {
	ap := NewArgParser("send", args)
	value := ap.Any("value")
	timeout, hasTimeout := ap.Timeout("timeout")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	defer func() {
		if recover() != nil {
			err = NewValueError("send on closed channel")
		}
	}()
	if !hasTimeout {
		c.ch <- value
		return Nil, nil
	}
	// Try first, so a ready channel is never lost to an expired timer: a Go
	// select picks at random among ready cases.
	select {
	case c.ch <- value:
		return Nil, nil
	default:
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case c.ch <- value:
		return Nil, nil
	case <-timer.C:
		return nil, NewTimeoutError("send() timed out after %g seconds", timeout.Seconds())
	}
}

// Receive blocks until a value is available, or until the optional timeout
// in seconds expires, which raises TimeoutError. A value that is already
// available is always received, even with timeout=0. When the channel is
// closed and drained it returns an error so the caller can distinguish it
// from a real nil value being sent.
func (c *Chan) Receive(args CallArgs) (Object, error) {
	ap := NewArgParser("receive", args)
	timeout, hasTimeout := ap.Timeout("timeout")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	var value Object
	var ok bool
	if !hasTimeout {
		value, ok = <-c.ch
	} else {
		select {
		case value, ok = <-c.ch:
		default:
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			select {
			case value, ok = <-c.ch:
			case <-timer.C:
				return nil, NewTimeoutError("receive() timed out after %g seconds", timeout.Seconds())
			}
		}
	}
	if !ok {
		return nil, NewValueError("receive on closed channel")
	}
	return value, nil
}

// Close closes the channel. Closing an already-closed channel returns an error
// instead of panicking.
func (c *Chan) Close(args CallArgs) (_ Object, err error) {
	if err := RequireNoArgs("close", args); err != nil {
		return nil, err
	}
	defer func() {
		if recover() != nil {
			err = NewValueError("close of closed channel")
		}
	}()
	close(c.ch)
	return Nil, nil
}

var ChanConstructorFn = &Function{Name: "Chan", Fn: ChanConstructor}

// ChanConstructor implements Chan(capacity=0): an unbuffered channel, or one
// buffering up to capacity values.
func ChanConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Chan", args)
	capacity := ap.IntOr("capacity", 0)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if capacity < 0 {
		return nil, NewValueError("Chan() capacity must be non-negative, got %d", int64(capacity))
	}
	return NewChan(int(capacity)), nil
}
