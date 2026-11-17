package http

import (
	"bufio"
	"bytes"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/jsonvalue"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

// Response is the status, the headers and the body of an answer, and is
// itself the body as a readable stream. The connection is released when the
// body has been read to its end or close() is called.
type Response struct {
	object.OpaqueBase
	resp    *stdhttp.Response
	url     *urlvalue.URL
	timeout time.Duration
	limited bool

	mu       sync.Mutex
	body     *autoBody
	reader   *bufio.Reader
	headers  *object.Dict
	userShut bool
	lineNo   int
}

var _ object.Object = (*Response)(nil)

func newResponse(resp *stdhttp.Response, timeout time.Duration, limited bool) *Response {
	r := &Response{
		OpaqueBase: object.MakeOpaqueBase("Response"),
		resp:       resp,
		url:        urlvalue.New(resp.Request.URL),
		timeout:    timeout,
		limited:    limited,
	}
	r.setBody(resp.Body)
	return r
}

func (r *Response) setBody(rc io.ReadCloser) {
	r.body = &autoBody{rc: rc}
	r.reader = bufio.NewReader(r.body)
}

// autoBody closes the body as soon as its end is reached, which releases
// the connection without the caller having to close the Response.
type autoBody struct {
	rc    io.ReadCloser
	ended bool
	shut  bool
}

func (b *autoBody) Read(p []byte) (int, error) {
	if b.shut {
		return 0, io.EOF
	}
	n, err := b.rc.Read(p)
	if err == io.EOF {
		b.ended = true
		b.close()
	}
	return n, err
}

func (b *autoBody) close() {
	if !b.shut {
		b.shut = true
		b.rc.Close()
	}
}

// statusError reads the rest of the body into memory, releases the
// connection, and builds the StatusError that carries this Response.
func (r *Response) statusError(fn, requested string) error {
	data, _ := io.ReadAll(r.reader)
	r.body.close()
	r.setBody(io.NopCloser(bytes.NewReader(data)))
	return object.NewKindError(StatusError, "%s() %s returned %d %s", fn, requested, r.resp.StatusCode, r.statusText()).
		With("status", object.Integer(r.resp.StatusCode)).
		With("response", r)
}

func (r *Response) statusText() string {
	text := strings.TrimPrefix(r.resp.Status, strconv.Itoa(r.resp.StatusCode))
	text = strings.TrimSpace(text)
	if text == "" {
		text = stdhttp.StatusText(r.resp.StatusCode)
	}
	return text
}

func (r *Response) Constructor() *object.Function { return responseType.Function }

func (r *Response) String() string {
	return "<Response " + strconv.Itoa(r.resp.StatusCode) + " " + r.statusText() + ">"
}

func (r *Response) ToString() (string, error) { return r.String(), nil }

// atEnd reports whether everything the body had has been handed out.
func (r *Response) atEnd() bool {
	return r.body.ended && r.reader.Buffered() == 0
}

// readError reports a failure to read the body as raised by method ("" for
// a for loop).
func (r *Response) readError(method string, err error) error {
	prefix := ""
	if method != "" {
		prefix = method + "() "
	}
	if isTimeout(err) {
		return object.NewTimeoutError("%stimed out after %g seconds", prefix, r.timeout.Seconds())
	}
	return object.WrapError(object.NetworkError, prefix+"cannot read the response body from "+r.resp.Request.URL.Host, err)
}

func (r *Response) read(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("read", args)
	size, hasSize := p.NullableInt("size")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if hasSize && size < 0 {
		return nil, object.NewValueError("read() argument 'size' must be non-negative, got %d", int64(size))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.userShut {
		return nil, object.NewValueError("read() called on closed response")
	}
	if !hasSize {
		data, err := io.ReadAll(r.reader)
		if err != nil {
			return nil, r.readError("read", err)
		}
		return object.NewBytes(data), nil
	}
	if size == 0 {
		return object.NewBytes(nil), nil
	}
	buf := make([]byte, int(size))
	for {
		n, err := r.reader.Read(buf)
		if n > 0 || err == io.EOF {
			return object.NewBytes(buf[:n]), nil
		}
		if err != nil {
			return nil, r.readError("read", err)
		}
	}
}

// rest reads the rest of the body for text() and json(), which refuse a
// body that was already read to its end.
func (r *Response) rest(method string) ([]byte, error) {
	if r.userShut {
		return nil, object.NewValueError("%s() called on closed response", method)
	}
	if r.atEnd() {
		return nil, object.NewValueError("%s() response body was already read", method)
	}
	data, err := io.ReadAll(r.reader)
	if err != nil {
		return nil, r.readError(method, err)
	}
	return data, nil
}

func (r *Response) text(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("text", args); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := r.rest("text")
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, object.NewValueError("text() response body is not valid UTF-8")
	}
	return object.String(data), nil
}

func (r *Response) json(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("json", args); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := r.rest("json")
	if err != nil {
		return nil, err
	}
	return jsonvalue.Decode("json", data)
}

func (r *Response) close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("close", args); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userShut = true
	r.body.close()
	return object.Nil, nil
}

func (r *Response) headerDict() *object.Dict {
	if r.headers != nil {
		return r.headers
	}
	names := make([]string, 0, len(r.resp.Header))
	for name := range r.resp.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	d := object.NewDict()
	for _, name := range names {
		d.Set(object.String(strings.ToLower(name)), object.String(strings.Join(r.resp.Header[name], ", ")))
	}
	r.headers = d
	return d
}

func (r *Response) headerValues(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("header_values", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	values := r.resp.Header.Values(string(name))
	out := make([]object.Object, len(values))
	for i, v := range values {
		out[i] = object.String(v)
	}
	return &object.List{Elements: out}, nil
}

// Iter returns the rest of the body as lines; see Iterator.
func (r *Response) Iter() ([]object.Object, error) { return object.Collect(r) }

// Iterator makes `for line in response` read the rest of the body one line
// at a time as it arrives. The loop does not own the Response: leaving it
// early leaves the rest unread and the connection held until close().
func (r *Response) Iterator() (object.Iterator, error) {
	r.mu.Lock()
	shut := r.userShut
	r.mu.Unlock()
	if shut {
		return nil, object.NewValueError("cannot iterate a closed response")
	}
	return object.IteratorFunc(func() (object.Object, bool, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.userShut {
			return nil, false, object.NewValueError("cannot iterate a closed response")
		}
		line, ok, err := object.ReadLine(r.reader)
		if err != nil {
			return nil, false, r.readError("", err)
		}
		if !ok {
			return nil, false, nil
		}
		r.lineNo++
		if !utf8.ValidString(line) {
			return nil, false, object.NewValueError("line %d of the response body is not valid UTF-8", r.lineNo).
				With("line", object.Integer(r.lineNo))
		}
		return object.String(line), true, nil
	}), nil
}

func (r *Response) GetAttr(name string) (object.Object, error) {
	switch name {
	case "status":
		return object.Integer(r.resp.StatusCode), nil
	case "status_text":
		return object.String(r.statusText()), nil
	case "success":
		return object.Bool(r.resp.StatusCode < 400), nil
	case "url":
		return r.url, nil
	case "headers":
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.headerDict(), nil
	case "closed":
		r.mu.Lock()
		defer r.mu.Unlock()
		return object.Bool(r.userShut || r.atEnd()), nil
	case "header_values":
		return &object.Function{Name: "header_values", Fn: r.headerValues}, nil
	case "text":
		return &object.Function{Name: "text", Fn: r.text}, nil
	case "json":
		return &object.Function{Name: "json", Fn: r.json}, nil
	case "read":
		return &object.Function{Name: "read", Fn: r.read}, nil
	case "close":
		return &object.Function{Name: "close", Fn: r.close}, nil
	}
	return nil, object.NewAttributeError("Response has no attribute '%s'", name)
}

func (r *Response) Attributes() []string {
	return []string{"status", "status_text", "success", "url", "headers", "header_values", "text", "json", "read", "close", "closed"}
}
