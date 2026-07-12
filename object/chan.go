package object

// Chan wraps a Go channel of Objects, exposing send/recv/close to Goblin.
// Element typing is dynamic: any Object can flow through the channel.
type Chan struct {
	ch chan Object
}

var _ Object = &Chan{}

func (c *Chan) String() string { return "<chan>" }

func (c *Chan) ToString() (string, error) { return c.String(), nil }
func (c *Chan) Bool() bool                { return true }

func (c *Chan) Compare(other Object) (int, error) {
	if o, ok := other.(*Chan); ok {
		if c == o {
			return 0, nil
		}
		return 1, nil
	}
	return 0, NewTypeError("cannot compare Chan and %T", other)
}

func (c *Chan) Add(Object) (Object, error)      { return nil, NewTypeError("cannot add Chan") }
func (c *Chan) Minus(Object) (Object, error)    { return nil, NewTypeError("cannot subtract Chan") }
func (c *Chan) Multiply(Object) (Object, error) { return nil, NewTypeError("cannot multiply Chan") }
func (c *Chan) Divide(Object) (Object, error)   { return nil, NewTypeError("cannot divide Chan") }
func (c *Chan) And(Object) (Object, error)      { return nil, NewTypeError("cannot perform AND on Chan") }
func (c *Chan) Or(Object) (Object, error)       { return nil, NewTypeError("cannot perform OR on Chan") }
func (c *Chan) Not() (Object, error)            { return nil, NewTypeError("cannot perform NOT on Chan") }
func (c *Chan) Iter() ([]Object, error) {
	return nil, NewTypeError("Chan does not support iteration")
}
func (c *Chan) Index(Object) (Object, error) { return nil, NewTypeError("Chan is not indexable") }

func (c *Chan) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(c), nil
	case "send":
		return &Function{Name: "send", Fn: c.Send}, nil
	case "recv":
		return &Function{Name: "recv", Fn: c.Recv}, nil
	case "close":
		return &Function{Name: "close", Fn: c.Close}, nil
	case "constructor":
		return ChanConstructorFn, nil
	default:
		return nil, NewAttributeError("Chan has no attribute '%s'", name)
	}
}

func (c *Chan) Attributes() []string {
	return []string{"attributes", "send", "recv", "close", "constructor"}
}

// Send blocks until the value is delivered or buffered. Sending on a closed
// channel returns an error instead of panicking.
func (c *Chan) Send(args CallArgs) (_ Object, err error) {
	if err := RequireNoKeyword("send", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, NewTypeError("send() takes exactly 1 argument, got %d", len(args.Positional))
	}
	defer func() {
		if recover() != nil {
			err = NewValueError("send on closed channel")
		}
	}()
	c.ch <- args.Positional[0]
	return Nil, nil
}

// Recv blocks until a value is available. When the channel is closed and
// drained it returns an error so the caller can distinguish it from a real
// nil value being sent.
func (c *Chan) Recv(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("recv", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("recv() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	value, ok := <-c.ch
	if !ok {
		return nil, NewValueError("recv on closed channel")
	}
	return value, nil
}

// Close closes the channel. Closing an already-closed channel returns an error
// instead of panicking.
func (c *Chan) Close(args CallArgs) (_ Object, err error) {
	if err := RequireNoKeyword("close", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, NewTypeError("close() takes exactly 0 arguments, got %d", len(args.Positional))
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

// ChanConstructor builds a Chan. With no argument it is unbuffered; with a
// single Integer argument that integer is the buffer size.
func ChanConstructor(args CallArgs) (Object, error) {
	ap := NewArgParser("Chan", args)
	size := ap.IntOr("size", 0)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, NewValueError("Chan() size must be non-negative, got %d", int64(size))
	}
	return &Chan{ch: make(chan Object, int(size))}, nil
}
