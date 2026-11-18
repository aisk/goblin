package object

import "io"

// This file implements the stream shapes of STDLIB_DESIGN.md §2, which are
// library protocols: a reader is anything with a read(size) method, a writer
// anything with a write(data) method, and close() is optional for both.
// DuckReader and DuckWriter adapt them to io.ReadCloser / io.WriteCloser so
// Go-implemented stdlib modules can consume them.

// streamMethods resolves value's stream method and its optional close.
func streamMethods(fn, param string, value Object, method, shape string) (call, closer *Function, err error) {
	call, err = duckMethod(fn, param, value, method, shape)
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
func duckMethod(fn, param string, value Object, name, shape string) (*Function, error) {
	obj, err := value.GetAttr(name)
	if err != nil {
		return nil, NewTypeError("%s() argument '%s' must have a %s method, got %s", fn, param, shape, value.TypeName())
	}
	method, ok := AsFunction(obj)
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
	method, ok := AsFunction(obj)
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
	readFn, closeFn, err := streamMethods(fn, param, value, "read", "read(size)")
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
			return 0, NewTypeError("%s() argument '%s' read(size) must return Bytes, String, or Nil, got %s", r.fn, r.param, value.TypeName())
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
	writeFn, closeFn, err := streamMethods(fn, param, value, "write", "write(data)")
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
			return written, NewTypeError("%s() argument '%s' write(data) must return Int or Nil, got %s", w.fn, w.param, value.TypeName())
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
