// Package http is the http module, an HTTP client: one call for a request,
// keywords for what the request carries, and the Response as the body
// stream. Anything that is a policy (a base URL, retrying, how a file
// becomes a form) is left to the caller, who has the pieces for it.
package http

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"time"

	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/jsonvalue"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

// StatusError is raised for a status of 400 or above under check=true.
// Data: status, response.
var StatusError = object.NewErrorKind("StatusError", object.BaseError)

// userAgent is sent unless headers= names one.
const userAgent = "goblin"

// defaultTimeout bounds a whole exchange unless timeout= says otherwise.
const defaultTimeout = 30 * time.Second

var (
	responseType = object.NewOpaqueConstructor("Response", "http.get(url)")
	clientType   = object.NewNativeConstructor("Client", newClient)
)

func Execute() (object.Object, error) {
	return object.NewModule("http", map[string]object.Object{
		"Client":      clientType.Function,
		"Response":    responseType.Function,
		"StatusError": StatusError,
	}, object.Funcs{
		"get":       moduleCall("get", "GET", false),
		"delete":    moduleCall("delete", "DELETE", false),
		"post":      moduleCall("post", "POST", true),
		"put":       moduleCall("put", "PUT", true),
		"patch":     moduleCall("patch", "PATCH", true),
		"request":   moduleCall("request", "", true),
		"multipart": multipartBody,
	}), nil
}

// The verifying transport is Go's default; the insecure one is a clone of
// it that accepts any certificate, made on first use. Their connection
// pools are separate, so a connection opened without verification is never
// reused by a verifying client.
var (
	insecureOnce      sync.Once
	insecureTransport *stdhttp.Transport
)

func transport(insecure bool) stdhttp.RoundTripper {
	if !insecure {
		return stdhttp.DefaultTransport
	}
	insecureOnce.Do(func() {
		t := stdhttp.DefaultTransport.(*stdhttp.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		insecureTransport = t
	})
	return insecureTransport
}

// session is what a request is sent with: the Client's settings, or for a
// module function the defaults and a jar of its own.
type session struct {
	headers   *object.Dict
	timeout   time.Duration
	limited   bool
	jar       stdhttp.CookieJar
	insecure  bool
	redirects bool
}

// moduleCall is a module-level request function: each call keeps the
// cookies it is given for its own redirect chain and forgets them after.
func moduleCall(fn, method string, withBody bool) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		jar, _ := cookiejar.New(nil)
		s := &session{timeout: defaultTimeout, limited: true, jar: jar, redirects: true}
		return s.call(fn, method, withBody, args)
	}
}

// request is one parsed call.
type request struct {
	fn      string
	method  string
	url     *url.URL
	params  *object.Dict
	headers *object.Dict
	body    object.Object
	json    object.Object
	form    *object.Dict
	timeout time.Duration
	limited bool
	check   bool
}

// call parses the arguments of fn in their documented order and sends the
// request. method is "" for request(), which takes it as its first argument.
func (s *session) call(fn, method string, withBody bool, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(fn, args)
	r := &request{fn: fn, method: method}
	if method == "" {
		r.method = string(p.Str("method"))
	}
	rawURL := p.Any("url")
	var bodyGiven, jsonGiven, formGiven bool
	var formObj object.Object
	if withBody {
		r.body, bodyGiven = p.OptionalAny("body")
		r.json, jsonGiven = p.OptionalAny("json")
		formObj, formGiven = p.OptionalAny("form")
	}
	paramsObj := p.AnyOr("params", object.Nil)
	headersObj := p.AnyOr("headers", object.Nil)
	timeout, limited, given := p.OptionalTimeout("timeout")
	r.check = bool(p.BoolOr("check", true))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if given {
		r.timeout, r.limited = timeout, limited
	} else {
		r.timeout, r.limited = s.timeout, s.limited
	}
	bodies := 0
	for _, v := range []struct {
		given bool
		value object.Object
	}{{bodyGiven, r.body}, {jsonGiven, r.json}, {formGiven, formObj}} {
		if v.given && v.value != object.Nil {
			bodies++
		}
	}
	if bodies > 1 {
		return nil, object.NewValueError("%s() takes at most one of 'body', 'json' and 'form'", fn)
	}
	if !bodyGiven {
		r.body = object.Nil
	}
	if !jsonGiven {
		r.json = object.Nil
	}
	var err error
	if r.params, err = optionalDict(fn, "params", paramsObj); err != nil {
		return nil, err
	}
	if r.form, err = optionalDict(fn, "form", formObj); err != nil {
		return nil, err
	}
	if r.headers, err = headerDict(fn, "headers", headersObj); err != nil {
		return nil, err
	}
	if r.url, err = requestURL(fn, rawURL); err != nil {
		return nil, err
	}
	return s.send(r)
}

func optionalDict(fn, param string, v object.Object) (*object.Dict, error) {
	if v == nil || v == object.Nil {
		return nil, nil
	}
	d, ok := v.(*object.Dict)
	if !ok {
		return nil, object.NewTypeError("%s() argument '%s' must be Dict or Nil, got %s", fn, param, v.TypeName())
	}
	return d, nil
}

// headerDict checks a headers= Dict: String names, String or List of
// String values, and none of the headers the library frames itself.
func headerDict(fn, param string, v object.Object) (*object.Dict, error) {
	d, err := optionalDict(fn, param, v)
	if err != nil || d == nil {
		return nil, err
	}
	for _, entry := range d.Entries() {
		name, ok := entry.Key.(object.String)
		if !ok {
			return nil, object.NewTypeError("%s() argument '%s' keys must be String, got %s", fn, param, entry.Key.TypeName())
		}
		if _, err := headerValues(fn, param, entry.Value); err != nil {
			return nil, err
		}
		switch textproto.CanonicalMIMEHeaderKey(string(name)) {
		case "Content-Length", "Transfer-Encoding":
			return nil, object.NewValueError("%s() argument '%s' must not set %s; the library frames the body", fn, param, textproto.CanonicalMIMEHeaderKey(string(name)))
		}
	}
	return d, nil
}

func headerValues(fn, param string, v object.Object) ([]string, error) {
	switch value := v.(type) {
	case object.String:
		return []string{string(value)}, nil
	case *object.List:
		out := make([]string, len(value.Elements))
		for i, item := range value.Elements {
			s, ok := item.(object.String)
			if !ok {
				return nil, object.NewTypeError("%s() argument '%s' values must be String or List of String, got List containing %s", fn, param, item.TypeName())
			}
			out[i] = string(s)
		}
		return out, nil
	}
	return nil, object.NewTypeError("%s() argument '%s' values must be String or List of String, got %s", fn, param, v.TypeName())
}

// requestURL is the absolute http or https URL a url argument names.
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
	return u, nil
}

// applyHeaders sets each header of d on h, replacing any earlier value of
// the same name whatever its case; a List value adds the header once per
// element. Host is the request's host, not a header Go would send.
func applyHeaders(req *stdhttp.Request, d *object.Dict) {
	if d == nil {
		return
	}
	for _, entry := range d.Entries() {
		name := string(entry.Key.(object.String))
		values, _ := headerValues("", "", entry.Value)
		if textproto.CanonicalMIMEHeaderKey(name) == "Host" {
			if len(values) > 0 {
				req.Host = values[len(values)-1]
			}
			continue
		}
		req.Header.Del(name)
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
}

// send builds and sends one request, exactly once.
func (s *session) send(r *request) (object.Object, error) {
	u := *r.url
	if r.params != nil {
		query, err := urlvalue.EncodeQuery(r.fn, "params", r.params)
		if err != nil {
			return nil, err
		}
		if query != "" {
			if u.RawQuery != "" {
				u.RawQuery += "&" + query
			} else {
				u.RawQuery = query
			}
		}
	}
	body, err := r.makeBody()
	if err != nil {
		return nil, err
	}
	req, err := stdhttp.NewRequest(r.method, u.String(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "invalid method") {
			return nil, object.NewValueError("%s() invalid method %q", r.fn, r.method)
		}
		return nil, object.WrapError(object.ValueError, r.fn+"() cannot build the request", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if body.contentType != "" {
		req.Header.Set("Content-Type", body.contentType)
	}
	body.attach(req)
	applyHeaders(req, s.headers)
	applyHeaders(req, r.headers)

	client := &stdhttp.Client{Transport: transport(s.insecure), Jar: s.jar}
	if r.limited {
		client.Timeout = r.timeout
	}
	if !s.redirects {
		client.CheckRedirect = func(*stdhttp.Request, []*stdhttp.Request) error { return stdhttp.ErrUseLastResponse }
	}
	resp, err := client.Do(req)
	// From here on nothing reads the caller's stream: a transport that is
	// still writing gets an error instead of calling into Goblin code.
	raised := body.finish()
	if err != nil {
		if raised != nil {
			return nil, raised
		}
		return nil, r.exchangeError(&u, err)
	}
	response := newResponse(resp, r.timeout, r.limited)
	if r.check && resp.StatusCode >= 400 {
		return nil, response.statusError(r.fn, u.String())
	}
	return response, nil
}

// exchangeError reports a request that got no response.
func (r *request) exchangeError(u *url.URL, err error) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) {
		return err
	}
	if isTimeout(err) {
		return timeoutError(r.fn, r.timeout)
	}
	cause := err
	var ue *url.Error
	if errors.As(err, &ue) {
		cause = ue.Err
	}
	// Go reports a body that turned out shorter or longer than the length
	// measured when the request started only in its message.
	if strings.Contains(cause.Error(), "ContentLength=") {
		return object.WrapError(object.NetworkError, r.fn+"() the body changed size while it was sent to "+u.Host, cause)
	}
	return object.WrapError(object.NetworkError, r.fn+"() cannot reach "+u.Host, cause)
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

func timeoutError(fn string, timeout time.Duration) error {
	return object.NewTimeoutError("%s() timed out after %g seconds", fn, timeout.Seconds())
}

// jsonBody encodes a json= value.
func jsonBody(fn string, v object.Object) ([]byte, error) {
	text, err := jsonvalue.Encode(fn, v, 0, false)
	return []byte(text), err
}

// fileOf returns the File a body or part is, if it is one.
func fileOf(v object.Object) (*filestream.File, bool) {
	f, ok := v.(*filestream.File)
	return f, ok
}

var _ io.Reader = (*recordingReader)(nil)
