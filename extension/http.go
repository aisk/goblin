package extension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aisk/goblin/object"
	"github.com/pkg/errors"
)

// defaultTimeout is used when the caller does not specify a timeout.  A finite
// value prevents an unresponsive server from hanging the interpreter forever.
const defaultTimeout = 30 * time.Second

// ---------------------------------------------------------------------------
// HTTPResponse
// ---------------------------------------------------------------------------

type HTTPResponse struct {
	Status  int
	Headers *object.Dict
	body    []byte
}

func NewHTTPResponse(status int, headers *object.Dict, body []byte) *HTTPResponse {
	return &HTTPResponse{
		Status:  status,
		Headers: headers,
		body:    body,
	}
}

func (r *HTTPResponse) String() string {
	return fmt.Sprintf("<http_response %d>", r.Status)
}
func (r *HTTPResponse) Bool() bool { return true }
func (r *HTTPResponse) Compare(object.Object) (int, error) {
	return 0, fmt.Errorf("cannot compare HTTPResponse")
}
func (r *HTTPResponse) Add(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot add HTTPResponse")
}
func (r *HTTPResponse) Minus(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot subtract HTTPResponse")
}
func (r *HTTPResponse) Multiply(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot multiply HTTPResponse")
}
func (r *HTTPResponse) Divide(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot divide HTTPResponse")
}
func (r *HTTPResponse) And(other object.Object) (object.Object, error) {
	return object.Bool(r.Bool() && other.Bool()), nil
}
func (r *HTTPResponse) Or(other object.Object) (object.Object, error) {
	return object.Bool(r.Bool() || other.Bool()), nil
}
func (r *HTTPResponse) Not() (object.Object, error) { return object.Bool(!r.Bool()), nil }
func (r *HTTPResponse) Iter() ([]object.Object, error) {
	return nil, fmt.Errorf("HTTPResponse does not support iteration")
}
func (r *HTTPResponse) Index(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("HTTPResponse is not indexable")
}
func (r *HTTPResponse) GetAttr(name string) (object.Object, error) {
	switch name {
	case "status":
		return object.Integer(r.Status), nil
	case "headers":
		return r.Headers, nil
	case "body":
		return object.String(string(r.body)), nil
	case "ok":
		return object.Bool(r.Status >= 200 && r.Status < 300), nil
	case "text":
		return &object.Function{Name: "text", Fn: r.Text}, nil
	case "json":
		return &object.Function{Name: "json", Fn: r.JSON}, nil
	default:
		return nil, fmt.Errorf("HTTPResponse has no attribute '%s'", name)
	}
}

// Text returns the response body as a string.
func (r *HTTPResponse) Text(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("text", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("text() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return object.String(string(r.body)), nil
}

// JSON parses the response body as JSON.
func (r *HTTPResponse) JSON(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("json", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("json() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	dec := json.NewDecoder(bytes.NewReader(r.body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("json() failed to parse response body: %w", err)
	}
	return jsonToGoblin(v)
}

// ---------------------------------------------------------------------------
// Module entry point
// ---------------------------------------------------------------------------

func ExecuteHttp() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"request": &object.Function{Name: "request", Fn: httpRequest},
			"get":     &object.Function{Name: "get", Fn: httpGet},
			"post":    &object.Function{Name: "post", Fn: httpPost},
			"put":     &object.Function{Name: "put", Fn: httpPut},
			"delete":  &object.Function{Name: "delete", Fn: httpDelete},
			"patch":   &object.Function{Name: "patch", Fn: httpPatch},
			"head":    &object.Function{Name: "head", Fn: httpHead},
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Convenience methods — inject the HTTP method and delegate to httpRequest.
// ---------------------------------------------------------------------------

func httpGet(args object.CallArgs) (object.Object, error) {
	return httpWithMethod("GET", args)
}

func httpPost(args object.CallArgs) (object.Object, error) {
	return httpWithMethod("POST", args)
}

func httpPut(args object.CallArgs) (object.Object, error) {
	return httpWithMethod("PUT", args)
}

func httpDelete(args object.CallArgs) (object.Object, error) {
	return httpWithMethod("DELETE", args)
}

func httpPatch(args object.CallArgs) (object.Object, error) {
	return httpWithMethod("PATCH", args)
}

func httpHead(args object.CallArgs) (object.Object, error) {
	return httpWithMethod("HEAD", args)
}

func httpWithMethod(method string, args object.CallArgs) (object.Object, error) {
	// Prepend the method as the first positional argument so the existing
	// bindHTTPArgs logic handles everything uniformly.
	newArgs := object.CallArgs{
		Positional: append(object.Args{object.String(method)}, args.Positional...),
		Keyword:    args.Keyword,
	}
	return httpRequest(newArgs)
}

// ---------------------------------------------------------------------------
// Core request logic
// ---------------------------------------------------------------------------

type httpRequestOpts struct {
	method  string
	url     string
	body    string
	headers map[string]string
	params  map[string]string
	timeout time.Duration
}

func httpRequest(args object.CallArgs) (object.Object, error) {
	opts, err := bindHTTPArgs(args)
	if err != nil {
		return nil, err
	}

	fullURL := opts.url
	if len(opts.params) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, errors.Wrap(err, "request() invalid URL")
		}
		q := u.Query()
		for key, value := range opts.params {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	var reader io.Reader
	if opts.body != "" {
		reader = strings.NewReader(opts.body)
	}

	req, err := stdhttp.NewRequest(opts.method, fullURL, reader)
	if err != nil {
		return nil, errors.Wrap(err, "request() failed")
	}
	for key, value := range opts.headers {
		req.Header.Set(key, value)
	}

	timeout := opts.timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	client := &stdhttp.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request() failed")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "request() failed")
	}

	return NewHTTPResponse(resp.StatusCode, headersFromHTTP(resp.Header), data), nil
}

// ---------------------------------------------------------------------------
// Argument binding
// ---------------------------------------------------------------------------

func bindHTTPArgs(args object.CallArgs) (httpRequestOpts, error) {
	if len(args.Positional) > 4 {
		return httpRequestOpts{}, fmt.Errorf("request() takes at most 4 positional arguments, got %d", len(args.Positional))
	}

	values := map[string]object.Object{}
	set := func(name string, value object.Object) error {
		if _, exists := values[name]; exists {
			return fmt.Errorf("request() got multiple values for argument '%s'", name)
		}
		values[name] = value
		return nil
	}

	switch len(args.Positional) {
	case 1:
		if err := set("url", args.Positional[0]); err != nil {
			return httpRequestOpts{}, err
		}
	case 2, 3, 4:
		names := []string{"method", "url", "body", "headers"}
		for i, value := range args.Positional {
			if err := set(names[i], value); err != nil {
				return httpRequestOpts{}, err
			}
		}
	}

	for key, value := range args.Keyword {
		switch key {
		case "header":
			key = "headers"
			fallthrough
		case "method", "url", "body", "headers", "params", "timeout":
			if err := set(key, value); err != nil {
				return httpRequestOpts{}, err
			}
		default:
			return httpRequestOpts{}, fmt.Errorf("request() got an unexpected keyword argument '%s'", key)
		}
	}

	opts := httpRequestOpts{}

	// method
	opts.method = "GET"
	if value, ok := values["method"]; ok {
		methodObj, ok := value.(object.String)
		if !ok {
			return httpRequestOpts{}, fmt.Errorf("request() method argument must be a string, got %T", value)
		}
		opts.method = string(methodObj)
	}

	// url (required)
	urlObj, ok := values["url"]
	if !ok {
		return httpRequestOpts{}, fmt.Errorf("request() missing required argument: 'url'")
	}
	urlString, ok := urlObj.(object.String)
	if !ok {
		return httpRequestOpts{}, fmt.Errorf("request() url argument must be a string, got %T", urlObj)
	}
	opts.url = string(urlString)

	// body
	if value, ok := values["body"]; ok {
		switch v := value.(type) {
		case object.Unit:
			opts.body = ""
		case object.String:
			opts.body = string(v)
		default:
			return httpRequestOpts{}, fmt.Errorf("request() body argument must be a string or nil, got %T", value)
		}
	}

	// headers
	if value, ok := values["headers"]; ok {
		var err error
		opts.headers, err = stringMapFromObject("headers", value)
		if err != nil {
			return httpRequestOpts{}, err
		}
	}

	// params (query parameters)
	if value, ok := values["params"]; ok {
		var err error
		opts.params, err = stringMapFromObject("params", value)
		if err != nil {
			return httpRequestOpts{}, err
		}
	}

	// timeout
	if value, ok := values["timeout"]; ok {
		switch v := value.(type) {
		case object.Float:
			opts.timeout = time.Duration(float64(v) * float64(time.Second))
		case object.Integer:
			opts.timeout = time.Duration(int64(v)) * time.Second
		default:
			return httpRequestOpts{}, fmt.Errorf("request() timeout argument must be a number, got %T", value)
		}
	}

	return opts, nil
}

// stringMapFromObject converts a goblin dict (or nil) to map[string]string.
func stringMapFromObject(argName string, value object.Object) (map[string]string, error) {
	if _, ok := value.(object.Unit); ok {
		return map[string]string{}, nil
	}

	dict, ok := value.(*object.Dict)
	if !ok {
		return nil, fmt.Errorf("request() %s argument must be a dict or nil, got %T", argName, value)
	}

	result := make(map[string]string, len(dict.Entries))
	for _, entry := range dict.Entries {
		key, ok := entry.Key.(object.String)
		if !ok {
			return nil, fmt.Errorf("request() %s keys must be strings, got %T", argName, entry.Key)
		}
		val, ok := entry.Value.(object.String)
		if !ok {
			return nil, fmt.Errorf("request() %s values must be strings, got %T", argName, entry.Value)
		}
		result[string(key)] = string(val)
	}
	return result, nil
}

func headersFromHTTP(header stdhttp.Header) *object.Dict {
	result := object.NewDict()
	for key, values := range header {
		result.Set(object.String(key), object.String(strings.Join(values, ", ")))
	}
	return result
}

var _ object.Object = (*HTTPResponse)(nil)
