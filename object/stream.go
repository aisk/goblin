package object

import "io"

// This file implements the canonical stream shapes from STDLIB_DESIGN.md §5.
// A reader is anything with a read(size) method, a writer anything with a
// write(data) method, and close() is optional for both. A user type can
// provide the shape either as ordinary methods or by implementing the io
// module's Reader and Writer traits; DuckReader and DuckWriter accept both
// and adapt them to io.ReadCloser / io.WriteCloser so Go-implemented stdlib
// modules can consume them.

// Method indexes of the stream traits.
const (
	StreamIO = iota
	StreamClose
)

// ReaderTrait and WriterTrait are the stream shapes as traits, exported by the
// io module. On a value without an impl they call the value's own
// read/write/close methods, so io.Reader.read(file, 10) works on any stream.
var (
	ReaderTrait = newStreamTrait("Reader", "read")
	WriterTrait = newStreamTrait("Writer", "write")
)

func newStreamTrait(name, method string) *Trait {
	t := NewTrait(name, nil, []TraitMethod{
		{Name: method, Arity: 2, Required: true},
		// close is optional; a stream without one has nothing to release.
		{Name: "close", Arity: 1, Default: goFn("close", func([]Object) (Object, error) { return Nil, nil })},
	})
	t.duck = true
	t.route = []func([]Object) (Object, error){
		func(args []Object) (Object, error) {
			attr, err := args[0].GetAttr(method)
			if err != nil {
				return nil, NewTypeError(ErrFmtNotImplemented, args[0].TypeName(), name)
			}
			return Call(attr, CallArgs{Positional: args[1:]})
		},
		func(args []Object) (Object, error) {
			fn, err := duckCloseMethod(name+".close", "self", args[0])
			if err != nil || fn == nil {
				return Nil, err
			}
			return fn.Call(CallArgs{})
		},
	}
	return t
}

// streamMethods resolves value's stream method and its optional close: through
// the trait when value is a user type implementing it, else by attribute.
func streamMethods(fn, param string, value Object, trait *Trait, method, shape string) (call, closer *Function, err error) {
	if uv, ok := value.(UserValue); ok && uv.UserType().Impl(trait) != nil {
		bind := func(i int) *Function {
			return &Function{Name: trait.Name + "." + trait.Methods[i].Name, Fn: func(args CallArgs) (Object, error) {
				return trait.call(i, append([]Object{value}, args.Positional...))
			}}
		}
		return bind(StreamIO), bind(StreamClose), nil
	}
	call, err = duckMethod(fn, param, value, trait, method, shape)
	if err != nil {
		return nil, nil, err
	}
	closer, err = duckCloseMethod(fn, param, value)
	if err != nil {
		return nil, nil, err
	}
	return call, closer, nil
}

// duckMethod fetches a required stream method from value, rendering uniform
// TypeErrors on behalf of fn's parameter param. shape is the user-facing
// method signature, e.g. "read(size)".
func duckMethod(fn, param string, value Object, trait *Trait, name, shape string) (*Function, error) {
	obj, err := value.GetAttr(name)
	if err != nil {
		return nil, NewTypeError("%s() argument '%s' must implement io.%s or have a %s method, got %s", fn, param, trait.Name, shape, value.TypeName())
	}
	method, ok := obj.(*Function)
	if !ok {
		return nil, NewTypeError("%s() argument '%s' %s attribute must be callable, got %s", fn, param, name, obj.TypeName())
	}
	return method, nil
}

// duckCloseMethod fetches the optional close() method; a missing attribute is
// not an error, but a non-callable close attribute is.
func duckCloseMethod(fn, param string, value Object) (*Function, error) {
	obj, err := value.GetAttr("close")
	if err != nil {
		return nil, nil
	}
	method, ok := obj.(*Function)
	if !ok {
		return nil, NewTypeError("%s() argument '%s' close attribute must be callable, got %s", fn, param, obj.TypeName())
	}
	return method, nil
}

// DuckReader adapts any Goblin object with a read(size) method to
// io.ReadCloser. read(size) may return Bytes or str, and returns an empty
// chunk (or nil) at end of stream. A close() method is optional and is invoked
// by Close.
type DuckReader struct {
	fn      string
	param   string
	readFn  *Function
	closeFn *Function
	pending []byte
	eof     bool
	closed  bool
}

// NewDuckReader wraps value's read(size) method as an io.ReadCloser. fn and
// param name the calling function and its argument for error messages.
func NewDuckReader(fn, param string, value Object) (*DuckReader, error) {
	readFn, closeFn, err := streamMethods(fn, param, value, ReaderTrait, "read", "read(size)")
	if err != nil {
		return nil, err
	}
	return &DuckReader{fn: fn, param: param, readFn: readFn, closeFn: closeFn}, nil
}

func (r *DuckReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.pending) == 0 && !r.eof {
		value, err := r.readFn.Call(CallArgs{Positional: Args{Integer(len(p))}})
		if err != nil {
			return 0, err
		}
		switch v := value.(type) {
		case Unit:
			r.eof = true
		case Bytes:
			r.pending = append(r.pending, v...)
		case String:
			r.pending = append(r.pending, []byte(v)...)
		default:
			return 0, NewTypeError("%s() argument '%s' read(size) must return Bytes, str, or nil, got %s", r.fn, r.param, value.TypeName())
		}
		if len(r.pending) == 0 {
			r.eof = true
		}
	}
	if len(r.pending) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *DuckReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.closeFn == nil {
		return nil
	}
	_, err := r.closeFn.Call(CallArgs{})
	return err
}

// DuckWriter adapts any Goblin object with a write(data) method to
// io.WriteCloser. write(data) receives a Bytes chunk and returns the number of
// bytes written as an int; returning nil counts as the whole chunk. A close()
// method is optional and is invoked by Close — consumers that do not own the
// stream simply never call Close.
type DuckWriter struct {
	fn      string
	param   string
	writeFn *Function
	closeFn *Function
	closed  bool
}

// NewDuckWriter wraps value's write(data) method as an io.WriteCloser. fn and
// param name the calling function and its argument for error messages.
func NewDuckWriter(fn, param string, value Object) (*DuckWriter, error) {
	writeFn, closeFn, err := streamMethods(fn, param, value, WriterTrait, "write", "write(data)")
	if err != nil {
		return nil, err
	}
	return &DuckWriter{fn: fn, param: param, writeFn: writeFn, closeFn: closeFn}, nil
}

func (w *DuckWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	written := 0
	for written < len(p) {
		value, err := w.writeFn.Call(CallArgs{Positional: Args{NewBytes(p[written:])}})
		if err != nil {
			return written, err
		}
		switch v := value.(type) {
		case Unit:
			written = len(p)
		case Integer:
			remaining := len(p) - written
			if v < 0 || v > Integer(remaining) {
				return written, NewValueError("%s() argument '%s' write(data) returned %d for a %d-byte chunk", w.fn, w.param, int64(v), remaining)
			}
			if v == 0 {
				return written, io.ErrShortWrite
			}
			written += int(v)
		default:
			return written, NewTypeError("%s() argument '%s' write(data) must return int or nil, got %s", w.fn, w.param, value.TypeName())
		}
	}
	return len(p), nil
}

func (w *DuckWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if w.closeFn == nil {
		return nil
	}
	_, err := w.closeFn.Call(CallArgs{})
	return err
}

var (
	_ io.ReadCloser  = (*DuckReader)(nil)
	_ io.WriteCloser = (*DuckWriter)(nil)
)
