package http

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

var (
	requestType  = object.NewNativeConstructor("Request", newRequest)
	responseType = object.NewNativeConstructor("Response", newResponse)
)

// content is the body a message was built with: fixed bytes the message
// keeps (nil, String, Bytes), or a stream that can be sent once.
type content struct {
	body  *filestream.File
	data  []byte
	fixed bool
	// sent records that a stream body has been handed to an exchange.
	sent atomic.Bool
}

// makeContent turns a body= argument into a message body.
func makeContent(fn string, v object.Object) (*content, error) {
	switch value := v.(type) {
	case object.Unit:
		return fixedContent(nil), nil
	case object.String:
		return fixedContent([]byte(value)), nil
	case object.Bytes:
		return fixedContent([]byte(value)), nil
	case *filestream.File:
		return &content{body: value}, nil
	}
	r, ok := object.DataReader(fn, "body", v)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'body' must be String, Bytes, Nil or have a read(size) method, got %s", fn, v.TypeName())
	}
	// Closing this File does not close the readable: whoever opened it does.
	return &content{body: filestream.NewReader("body", r, nil)}, nil
}

func fixedContent(data []byte) *content {
	data = bytes.Clone(data)
	return &content{data: data, fixed: true, body: filestream.NewReader("body", bytes.NewReader(data), nil)}
}

// Request is a request message: method, URL, headers and body.
type Request struct {
	object.OpaqueBase
	method  string
	url     *url.URL
	headers *Headers
	content *content
}

var _ object.Object = (*Request)(nil)

// newRequest is Request(method, url, headers=nil, body=nil).
func newRequest(args object.CallArgs) (object.Object, error) {
	return parseRequest("Request", args)
}

// parseRequest reads method, url, headers=nil and body=nil as fn takes them
// and builds the Request.
func parseRequest(fn string, args object.CallArgs) (*Request, error) {
	p := object.NewArgParser(fn, args)
	method := p.Str("method")
	target := p.Any("url")
	headers := p.AnyOr("headers", object.Nil)
	body := p.AnyOr("body", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return buildRequest(fn, string(method), target, headers, body)
}

func buildRequest(fn, method string, target, headersObj, bodyObj object.Object) (*Request, error) {
	u, err := requestURL(fn, target)
	if err != nil {
		return nil, err
	}
	if !validName(method) {
		return nil, object.NewValueError("%s() invalid method %q", fn, method)
	}
	headers, err := headersFrom(fn, "headers", headersObj)
	if err != nil {
		return nil, err
	}
	c, err := makeContent(fn, bodyObj)
	if err != nil {
		return nil, err
	}
	return &Request{OpaqueBase: object.MakeOpaqueBase("Request"), method: method, url: u, headers: headers, content: c}, nil
}

// requestURL is the absolute http or https URL, without user information,
// that a url argument names.
func requestURL(fn string, v object.Object) (*url.URL, error) {
	u, err := urlvalue.FromObject(fn, "url", v)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, object.NewValueError("%s() needs an absolute URL, got %q", fn, u.String())
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, object.NewValueError("%s() URL scheme must be http or https, got %q", fn, u.Scheme)
	}
	if u.User != nil {
		return nil, object.NewValueError("%s() URL must not contain a user name or password; send an Authorization header", fn)
	}
	return u, nil
}

func (r *Request) Constructor() *object.Function { return requestType.Function }
func (r *Request) String() string                { return "<Request " + r.method + " " + r.url.String() + ">" }
func (r *Request) ToString() (string, error)     { return r.String(), nil }

func (r *Request) GetAttr(name string) (object.Object, error) {
	switch name {
	case "method":
		return object.String(r.method), nil
	case "url":
		return urlvalue.New(r.url), nil
	case "headers":
		return r.headers, nil
	case "body":
		return r.content.body, nil
	case "text":
		return &object.Function{Name: "text", Fn: bodyText(r.content.body)}, nil
	}
	return nil, object.NewAttributeError("Request has no attribute '%s'", name)
}

func (r *Request) Attributes() []string {
	return []string{"method", "url", "headers", "body", "text"}
}

// bodyText is text(): body.read().decode().
func bodyText(body *filestream.File) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		if err := object.RequireNoArgs("text", args); err != nil {
			return nil, err
		}
		data, err := body.Read(object.CallArgs{})
		if err != nil {
			return nil, err
		}
		b := data.(object.Bytes)
		if !utf8.Valid(b) {
			return nil, object.NewValueError("text() body is not valid UTF-8")
		}
		return object.String(b), nil
	}
}

// Response is a response message: status, headers and body, and for a
// received one the URL it answers and the HTTP version.
type Response struct {
	object.OpaqueBase
	status  int
	headers *Headers
	body    *filestream.File
	// url and version are those of the exchange; nil and "" when built.
	url     *url.URL
	version string
}

var _ object.Object = (*Response)(nil)

// newResponse is Response(status, headers=nil, body=nil).
func newResponse(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Response", args)
	status := p.Int("status")
	headersObj := p.AnyOr("headers", object.Nil)
	bodyObj := p.AnyOr("body", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if status < 100 || status > 999 {
		return nil, object.NewValueError("Response() argument 'status' must be from 100 to 999, got %d", int64(status))
	}
	headers, err := headersFrom("Response", "headers", headersObj)
	if err != nil {
		return nil, err
	}
	c, err := makeContent("Response", bodyObj)
	if err != nil {
		return nil, err
	}
	return &Response{OpaqueBase: object.MakeOpaqueBase("Response"), status: int(status), headers: headers, body: c.body}, nil
}

func (r *Response) Constructor() *object.Function { return responseType.Function }
func (r *Response) String() string                { return "<Response " + strconv.Itoa(r.status) + ">" }
func (r *Response) ToString() (string, error)     { return r.String(), nil }

func (r *Response) GetAttr(name string) (object.Object, error) {
	switch name {
	case "status":
		return object.Integer(r.status), nil
	case "headers":
		return r.headers, nil
	case "body":
		return r.body, nil
	case "url":
		if r.url == nil {
			return object.Nil, nil
		}
		return urlvalue.New(r.url), nil
	case "version":
		if r.version == "" {
			return object.Nil, nil
		}
		return object.String(r.version), nil
	case "text":
		return &object.Function{Name: "text", Fn: bodyText(r.body)}, nil
	}
	return nil, object.NewAttributeError("Response has no attribute '%s'", name)
}

func (r *Response) Attributes() []string {
	return []string{"status", "headers", "body", "url", "version", "text"}
}

// protoVersion is the version of a received response: "2" for HTTP/2,
// "1.1" or "1.0" otherwise.
func protoVersion(resp *stdhttp.Response) string {
	if resp.ProtoMajor >= 2 && resp.ProtoMinor == 0 {
		return strconv.Itoa(resp.ProtoMajor)
	}
	return strconv.Itoa(resp.ProtoMajor) + "." + strconv.Itoa(resp.ProtoMinor)
}

// receivedBody is the body of a received response as it arrives. It
// releases the connection at the first end of stream or on Close, bounds
// each wait for data by the call's timeout, and raises Goblin errors that
// name no function, since a read happens inside read(), a loop or a
// function that was given the body.
type receivedBody struct {
	mu      sync.Mutex
	raw     io.ReadCloser
	r       io.Reader
	gunzip  bool
	known   bool
	left    int64
	host    string
	dog     *watchdog
	release func()
	done    bool
}

func newReceivedBody(resp *stdhttp.Response, gunzip bool, dog *watchdog, release func()) *receivedBody {
	b := &receivedBody{raw: resp.Body, r: resp.Body, gunzip: gunzip, host: resp.Request.URL.Host, dog: dog, release: release}
	if !gunzip && resp.ContentLength >= 0 {
		b.known, b.left = true, resp.ContentLength
	}
	return b
}

// source is the reader the body comes from, decoding gzip when send asked
// for it; the gzip reader is made on the first read, and an empty body is
// an empty stream.
func (b *receivedBody) source() (io.Reader, error) {
	if b.gunzip {
		b.gunzip = false
		zr, err := gzip.NewReader(b.raw)
		if err == io.EOF {
			b.r = eofReader{}
		} else if err != nil {
			return nil, err
		} else {
			b.r = zr
		}
	}
	return b.r, nil
}

type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }

func (b *receivedBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return 0, io.EOF
	}
	b.dog.arm()
	r, err := b.source()
	n := 0
	if err == nil {
		n, err = r.Read(p)
	}
	fired := !b.dog.disarm()
	b.left -= int64(n)
	if err == io.EOF {
		b.finish()
		return n, io.EOF
	}
	if err != nil {
		b.finish()
		if fired {
			return n, object.NewTimeoutError("timed out after %s seconds waiting for the response body from %s", seconds(b.dog.d), b.host)
		}
		return n, object.WrapError(object.NetworkError, "cannot read the response body from "+b.host, err)
	}
	return n, nil
}

// readAll reads the rest within the call that is still running (for a
// StatusError): no timer of its own, and a failure keeps what arrived.
func (b *receivedBody) readAll() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return nil
	}
	var data []byte
	if r, err := b.source(); err == nil {
		data, _ = io.ReadAll(r)
	}
	b.finish()
	return data
}

func (b *receivedBody) finish() {
	if !b.done {
		b.done = true
		b.raw.Close()
		b.release()
	}
}

// Close abandons the rest of the body and releases the connection.
func (b *receivedBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.finish()
	return nil
}

// Remaining is the length still to come when the server declared one and
// the body is not being decoded, so a forwarded body keeps its length.
func (b *receivedBody) Remaining() (int64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.known || (b.done && b.left != 0) {
		return 0, false
	}
	return b.left, true
}

var _ filestream.Sized = (*receivedBody)(nil)

// receivedResponse builds the Response for resp. The body File releases
// the connection when it is collected unread.
func receivedResponse(resp *stdhttp.Response, gunzip bool, dog *watchdog, release func()) (*Response, *receivedBody) {
	headers := fromGo(resp.Header)
	if gunzip {
		headers.remove("content-encoding")
		headers.remove("content-length")
	}
	rb := newReceivedBody(resp, gunzip, dog, release)
	body := filestream.NewReader("body", rb, rb)
	runtime.AddCleanup(body, func(rb *receivedBody) { rb.Close() }, rb)
	return &Response{
		OpaqueBase: object.MakeOpaqueBase("Response"),
		status:     resp.StatusCode,
		headers:    headers,
		body:       body,
		url:        resp.Request.URL,
		version:    protoVersion(resp),
	}, rb
}

func seconds(d interface{ Seconds() float64 }) string {
	return strconv.FormatFloat(d.Seconds(), 'g', -1, 64)
}
