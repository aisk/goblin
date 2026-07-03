package http

import (
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
	if err := object.RequireNoKeyword("do", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, object.NewTypeError("do() takes exactly 1 argument, got %d", len(args.Positional))
	}
	reqObj, ok := args.Positional[0].(*Request)
	if !ok {
		return nil, object.NewTypeError("do() argument must be a request, got %T", args.Positional[0])
	}
	return doRequest(client, reqObj.Req)
}

func bodylessRequest(client *stdhttp.Client, fn, method string, args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword(fn, args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, object.NewTypeError("%s() takes exactly 1 argument, got %d", fn, len(args.Positional))
	}
	rawURL, err := stringArg(fn, "url", args.Positional[0])
	if err != nil {
		return nil, err
	}
	req, err := buildRequest(method, rawURL, "", "")
	if err != nil {
		return nil, object.WrapNativeError(object.NetworkError, fn+"() failed", err)
	}
	return doRequest(client, req)
}

func bodyRequest(client *stdhttp.Client, fn, method string, args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword(fn, args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 3 {
		return nil, object.NewTypeError("%s() takes exactly 3 arguments (url, content_type, body), got %d", fn, len(args.Positional))
	}
	rawURL, err := stringArg(fn, "url", args.Positional[0])
	if err != nil {
		return nil, err
	}
	contentType, err := stringArg(fn, "content_type", args.Positional[1])
	if err != nil {
		return nil, err
	}
	body, err := bodyArg(fn, args.Positional[2])
	if err != nil {
		return nil, err
	}
	req, err := buildRequest(method, rawURL, contentType, body)
	if err != nil {
		return nil, object.WrapNativeError(object.NetworkError, fn+"() failed", err)
	}
	return doRequest(client, req)
}

// ---------------------------------------------------------------------------
// Request / Client constructors
// ---------------------------------------------------------------------------

// newRequestObject implements the Request(method, url, body) constructor —
// mirrors http.NewRequest. The returned request is executed later via
// client.do.
func newRequestObject(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("Request", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 3 {
		return nil, object.NewTypeError("Request() takes exactly 3 arguments (method, url, body), got %d", len(args.Positional))
	}
	method, err := stringArg("Request", "method", args.Positional[0])
	if err != nil {
		return nil, err
	}
	rawURL, err := stringArg("Request", "url", args.Positional[1])
	if err != nil {
		return nil, err
	}
	body, err := bodyArg("Request", args.Positional[2])
	if err != nil {
		return nil, err
	}
	req, err := buildRequest(method, rawURL, "", body)
	if err != nil {
		return nil, object.WrapNativeError(object.NetworkError, "Request() failed", err)
	}
	return NewRequest(req), nil
}

// newClientObject implements the Client(timeout=seconds) constructor. timeout
// is accepted as a single positional argument or the "timeout" keyword; when
// omitted it defaults to defaultTimeout.
func newClientObject(args object.CallArgs) (object.Object, error) {
	if len(args.Positional) > 1 {
		return nil, object.NewTypeError("Client() takes at most 1 positional argument, got %d", len(args.Positional))
	}

	var timeoutObj object.Object
	if len(args.Positional) == 1 {
		timeoutObj = args.Positional[0]
	}
	for key, value := range args.Keyword {
		if key != "timeout" {
			return nil, object.NewTypeError("Client() got an unexpected keyword argument '%s'", key)
		}
		if timeoutObj != nil {
			return nil, object.NewTypeError("Client() got multiple values for argument 'timeout'")
		}
		timeoutObj = value
	}

	timeout := defaultTimeout
	if timeoutObj != nil {
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

// buildRequest constructs a *http.Request, wrapping a non-empty body string in
// a reader and setting Content-Type when provided.
func buildRequest(method, rawURL, contentType, body string) (*stdhttp.Request, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := stdhttp.NewRequest(method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// doRequest sends req using client, reads the full response body, and wraps the
// result in a Response.
func doRequest(client *stdhttp.Client, req *stdhttp.Request) (object.Object, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, object.WrapNativeError(object.NetworkError, "request failed", err)
	}
	defer resp.Body.Close()

	// TODO: buffering the entire body is unbounded; once goblin has a reader
	// type, expose a streaming body and/or cap this with an io.LimitReader.
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, object.WrapNativeError(object.NetworkError, "reading response body failed", err)
	}
	return NewResponse(resp, data), nil
}

// stringArg asserts that obj is a goblin string, returning a descriptive error
// otherwise.
func stringArg(fn, name string, obj object.Object) (string, error) {
	s, ok := obj.(object.String)
	if !ok {
		return "", object.NewTypeError("%s() %s argument must be a string, got %T", fn, name, obj)
	}
	return string(s), nil
}

// bodyArg accepts a string body or nil (meaning no body).
func bodyArg(fn string, obj object.Object) (string, error) {
	switch v := obj.(type) {
	case object.Unit:
		return "", nil
	case object.String:
		return string(v), nil
	default:
		return "", object.NewTypeError("%s() body argument must be a string or nil, got %T", fn, obj)
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
		return 0, object.NewTypeError("%s() timeout argument must be a number, got %T", fn, obj)
	}
}
