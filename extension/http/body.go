package http

import (
	"io"
	"sync/atomic"

	"github.com/aisk/goblin/object"
)

// Body is the native HTTP body stream. Its Goblin read(size=none) method
// returns Bytes; omitting size consumes the rest of the stream, while a size
// reads at most that many bytes. close() releases the underlying HTTP body.
//
// Body also implements io.ReadCloser so net/http and Goblin always operate on
// the same stream and observe the same close state.
type Body struct {
	objectBase
	stream io.ReadCloser
	closed atomic.Bool
}

func NewBody(stream io.ReadCloser) *Body {
	return &Body{objectBase: objectBase{typeName: "Body"}, stream: stream}
}

func (b *Body) String() string {
	if b.closed.Load() {
		return "<http_body closed>"
	}
	return "<http_body>"
}

func (b *Body) ToString() (string, error) { return b.String(), nil }

// Read implements io.Reader for net/http.
func (b *Body) Read(p []byte) (int, error) {
	if b.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	return b.stream.Read(p)
}

// Close implements io.Closer for net/http.
func (b *Body) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	return b.stream.Close()
}

func (b *Body) read(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("read", args)
	sizeObj := ap.AnyOr("size", object.Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if b.closed.Load() {
		return nil, object.NewValueError("read() called on closed HTTP body")
	}

	if _, ok := sizeObj.(object.Unit); ok {
		data, err := io.ReadAll(b)
		if err != nil {
			return nil, object.WrapNativeError(object.NetworkError, "reading HTTP body failed", err)
		}
		return object.NewBytes(data), nil
	}

	size, ok := sizeObj.(object.Integer)
	if !ok {
		return nil, object.NewTypeError("read() argument 'size' must be an int or nil, got %T", sizeObj)
	}
	if size < 0 {
		return nil, object.NewValueError("read() size must not be negative")
	}
	if size == 0 {
		return object.Bytes{}, nil
	}
	maxInt := int64(^uint(0) >> 1)
	if int64(size) > maxInt {
		return nil, object.NewValueError("read() size is too large")
	}

	data := make([]byte, int(size))
	n, err := b.Read(data)
	if err != nil && err != io.EOF {
		return nil, object.WrapNativeError(object.NetworkError, "reading HTTP body failed", err)
	}
	return object.NewBytes(data[:n]), nil
}

func (b *Body) close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("close", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("close() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	if err := b.Close(); err != nil {
		return nil, object.WrapNativeError(object.NetworkError, "closing HTTP body failed", err)
	}
	return object.Nil, nil
}

func (b *Body) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(b), nil
	case "read":
		return &object.Function{Name: "read", Fn: b.read}, nil
	case "close":
		return &object.Function{Name: "close", Fn: b.close}, nil
	case "closed":
		return object.Bool(b.closed.Load()), nil
	default:
		return nil, object.NewAttributeError("Body has no attribute '%s'", name)
	}
}

func (b *Body) Attributes() []string {
	return []string{"attributes", "read", "close", "closed"}
}

var (
	_ object.Object = (*Body)(nil)
	_ io.ReadCloser = (*Body)(nil)
)

// duckReader adapts any Goblin object with a read(size) method to
// io.ReadCloser. read(size) may return Bytes or str, and returns an empty value
// (or nil) at EOF. A close() method is optional and is invoked when net/http
// closes the request.
type duckReader struct {
	readFn  *object.Function
	closeFn *object.Function
	pending []byte
	eof     bool
	closed  bool
}

func newDuckReader(value object.Object) (*duckReader, error) {
	readObj, err := value.GetAttr("read")
	if err != nil {
		return nil, object.NewTypeError("HTTP body must be a string, Bytes, nil, or an object with a read(size) method, got %T", value)
	}
	readFn, ok := readObj.(*object.Function)
	if !ok {
		return nil, object.NewTypeError("HTTP body read attribute must be callable, got %T", readObj)
	}

	var closeFn *object.Function
	if closeObj, closeErr := value.GetAttr("close"); closeErr == nil {
		var ok bool
		closeFn, ok = closeObj.(*object.Function)
		if !ok {
			return nil, object.NewTypeError("HTTP body close attribute must be callable, got %T", closeObj)
		}
	}
	return &duckReader{readFn: readFn, closeFn: closeFn}, nil
}

func (r *duckReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.pending) == 0 && !r.eof {
		value, err := r.readFn.Call(object.CallArgs{Positional: object.Args{object.Integer(len(p))}})
		if err != nil {
			return 0, err
		}
		switch v := value.(type) {
		case object.Unit:
			r.eof = true
		case object.Bytes:
			r.pending = append(r.pending, v...)
		case object.String:
			r.pending = append(r.pending, []byte(v)...)
		default:
			return 0, object.NewTypeError("HTTP body read(size) must return Bytes, str, or nil, got %T", value)
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

func (r *duckReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.closeFn == nil {
		return nil
	}
	_, err := r.closeFn.Call(object.CallArgs{})
	return err
}

var _ io.ReadCloser = (*duckReader)(nil)
