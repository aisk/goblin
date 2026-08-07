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
	object.OpaqueBase
	stream io.ReadCloser
	closed atomic.Bool
}

func NewBody(stream io.ReadCloser) *Body {
	return &Body{OpaqueBase: object.MakeOpaqueBase("Body"), stream: stream}
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
			return nil, object.WrapNativeError(object.NetworkError, "read() failed to read HTTP body", err)
		}
		return object.NewBytes(data), nil
	}

	size, ok := sizeObj.(object.Integer)
	if !ok {
		return nil, object.NewTypeError("read() argument 'size' must be an int or nil, got %s", sizeObj.TypeName())
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
		return nil, object.WrapNativeError(object.NetworkError, "read() failed to read HTTP body", err)
	}
	return object.NewBytes(data[:n]), nil
}

func (b *Body) close(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("close", args).Finish(); err != nil {
		return nil, err
	}
	if err := b.Close(); err != nil {
		return nil, object.WrapNativeError(object.NetworkError, "close() failed to close HTTP body", err)
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
