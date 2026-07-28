package http

import (
	"bytes"
	"io"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/aisk/goblin/object"
)

// defaultTimeout is used by the module-level convenience functions and by
// http.Client() when no timeout is supplied. A finite value prevents an
// unresponsive server from hanging the interpreter forever.
const defaultTimeout = 30 * time.Second

// defaultClient backs the module-level convenience functions (get, post, ...),
// mirroring net/http.DefaultClient but with a safety timeout.
var defaultClient = &stdhttp.Client{Timeout: defaultTimeout}

// Execute builds the http module. The API mirrors Go's net/http naming in
// goblin's lowercase style: module-level get/head/post match http.Get/...,
// while put/patch/delete are convenience helpers modeled on Post. Request and
// Client are constructors for the corresponding types.
func Execute() (object.Object, error) {
	return &object.Module{
		Name: "http",
		Members: map[string]object.Object{
			"get":     &object.Function{Name: "get", Fn: get},
			"head":    &object.Function{Name: "head", Fn: head},
			"delete":  &object.Function{Name: "delete", Fn: deleteFn},
			"post":    &object.Function{Name: "post", Fn: post},
			"put":     &object.Function{Name: "put", Fn: put},
			"patch":   &object.Function{Name: "patch", Fn: patch},
			"Request": &object.Function{Name: "Request", Fn: newRequestObject},
			"Client":  &object.Function{Name: "Client", Fn: newClientObject},
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Module-level convenience functions (backed by defaultClient)
// ---------------------------------------------------------------------------

func get(args object.CallArgs) (object.Object, error)      { return doGet(defaultClient, args) }
func head(args object.CallArgs) (object.Object, error)     { return doHead(defaultClient, args) }
func deleteFn(args object.CallArgs) (object.Object, error) { return doDelete(defaultClient, args) }
func post(args object.CallArgs) (object.Object, error)     { return doPost(defaultClient, args) }
func put(args object.CallArgs) (object.Object, error)      { return doPut(defaultClient, args) }
func patch(args object.CallArgs) (object.Object, error)    { return doPatch(defaultClient, args) }

// ---------------------------------------------------------------------------
// Shared request logic — parameterized by the client so the same code serves
// both the module-level functions and the Client methods.
// ---------------------------------------------------------------------------

// doGet implements get(url) — mirrors http.Get / (*Client).Get.
func doGet(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	return bodylessRequest(client, "get", "GET", args)
}

// doHead implements head(url) — mirrors http.Head / (*Client).Head.
func doHead(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	return bodylessRequest(client, "head", "HEAD", args)
}

// doDelete implements delete(url). Go has no http.Delete; provided for
// convenience with a body-less signature.
func doDelete(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	return bodylessRequest(client, "delete", "DELETE", args)
}

// doPost implements post(url, content_type, body) — mirrors http.Post.
func doPost(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	return bodyRequest(client, "post", "POST", args)
}

// doPut implements put(url, content_type, body). Go has no http.Put; modeled on
// Post.
func doPut(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	return bodyRequest(client, "put", "PUT", args)
}

// doPatch implements patch(url, content_type, body). Go has no http.Patch;
// modeled on Post.
func doPatch(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	return bodyRequest(client, "patch", "PATCH", args)
}

// doDo implements do(request) — mirrors (*Client).Do.
func doDo(client *stdhttp.Client, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("do", args)
	requestArg := p.Any("request")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	reqObj, ok := requestArg.(*Request)
	if !ok {
		return nil, object.NewTypeError("do() argument 'request' must be Request, got %s", requestArg.TypeName())
	}
	return doRequest(client, "do", reqObj.Req)
}

func bodylessRequest(client *stdhttp.Client, fn, method string, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(fn, args)
	rawURL := p.Str("url")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	req, err := buildRequest(method, string(rawURL), "", nil)
	if err != nil {
		return nil, object.WrapError(object.ValueError, fn+"() invalid URL", err)
	}
	return doRequest(client, fn, req)
}

func bodyRequest(client *stdhttp.Client, fn, method string, args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(fn, args)
	rawURL, contentType := p.Str("url"), p.Str("content_type")
	bodyObj := p.Any("body")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	body, err := bodyArg(fn, bodyObj)
	if err != nil {
		return nil, err
	}
	req, err := buildRequest(method, string(rawURL), string(contentType), body)
	if err != nil {
		return nil, object.WrapError(object.ValueError, fn+"() invalid URL", err)
	}
	return doRequest(client, fn, req)
}

// ---------------------------------------------------------------------------
// Request / Client constructors
// ---------------------------------------------------------------------------

// newRequestObject implements the Request(method, url, body) constructor —
// mirrors http.NewRequest. The returned request is executed later via
// client.do.
func newRequestObject(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Request", args)
	method, rawURL := p.Str("method"), p.Str("url")
	bodyObj := p.AnyOr("body", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	body, err := bodyArg("Request", bodyObj)
	if err != nil {
		return nil, err
	}
	req, err := buildRequest(string(method), string(rawURL), "", body)
	if err != nil {
		return nil, object.WrapError(object.ValueError, "Request() invalid method or URL", err)
	}
	return NewRequest(req), nil
}

// newClientObject implements the Client(timeout=seconds) constructor. timeout
// is accepted as a single positional argument or the "timeout" keyword; when
// omitted it defaults to defaultTimeout.
func newClientObject(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Client", args)
	timeoutObj, hasTimeout := p.OptionalAny("timeout")
	if err := p.Finish(); err != nil {
		return nil, err
	}

	timeout := defaultTimeout
	if hasTimeout {
		var err error
		timeout, err = durationFromObject("Client", timeoutObj)
		if err != nil {
			return nil, err
		}
	}
	return NewClient(&stdhttp.Client{Timeout: timeout}), nil
}

// ---------------------------------------------------------------------------
// Low-level helpers
// ---------------------------------------------------------------------------

// buildRequest constructs a *http.Request and sets Content-Type when provided.
func buildRequest(method, rawURL, contentType string, body io.Reader) (*stdhttp.Request, error) {
	req, err := stdhttp.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// doRequest sends req using client and leaves the response body as a stream.
func doRequest(client *stdhttp.Client, fn string, req *stdhttp.Request) (object.Object, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, object.WrapNativeError(object.NetworkError, fn+"() request failed", err)
	}
	return NewResponse(resp), nil
}

// bodyArg accepts convenient in-memory values or any duck-typed object with a
// read(size) method. Duck readers may optionally expose close().
func bodyArg(fn string, obj object.Object) (io.Reader, error) {
	switch v := obj.(type) {
	case object.Unit:
		return nil, nil
	case object.String:
		return strings.NewReader(string(v)), nil
	case object.Bytes:
		return bytes.NewReader(v), nil
	default:
		reader, err := newDuckReader(fn, obj)
		if err != nil {
			return nil, err
		}
		return reader, nil
	}
}

// durationFromObject converts a number of seconds (int or float) to a Duration.
func durationFromObject(fn string, obj object.Object) (time.Duration, error) {
	switch v := obj.(type) {
	case object.Float:
		return time.Duration(float64(v) * float64(time.Second)), nil
	case object.Integer:
		return time.Duration(int64(v)) * time.Second, nil
	default:
		return 0, object.NewTypeError("%s() argument 'timeout' must be number, got %s", fn, obj.TypeName())
	}
}
