package extension

import (
	"fmt"
	"io"
	stdhttp "net/http"
	"strings"

	"github.com/aisk/goblin/object"
	"github.com/pkg/errors"
)

type HTTPBody struct {
	data []byte
}

func NewHTTPBody(data []byte) *HTTPBody {
	return &HTTPBody{data: data}
}

func (b *HTTPBody) Read(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("read", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("read() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	return object.String(string(b.data)), nil
}

func (b *HTTPBody) String() string { return "<http_body>" }
func (b *HTTPBody) Bool() bool     { return len(b.data) > 0 }
func (b *HTTPBody) Compare(object.Object) (int, error) {
	return 0, fmt.Errorf("cannot compare HTTPBody")
}
func (b *HTTPBody) Add(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot add HTTPBody")
}
func (b *HTTPBody) Minus(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot subtract HTTPBody")
}
func (b *HTTPBody) Multiply(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot multiply HTTPBody")
}
func (b *HTTPBody) Divide(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("cannot divide HTTPBody")
}
func (b *HTTPBody) And(other object.Object) (object.Object, error) {
	return object.Bool(b.Bool() && other.Bool()), nil
}
func (b *HTTPBody) Or(other object.Object) (object.Object, error) {
	return object.Bool(b.Bool() || other.Bool()), nil
}
func (b *HTTPBody) Not() (object.Object, error) { return object.Bool(!b.Bool()), nil }
func (b *HTTPBody) Iter() ([]object.Object, error) {
	return nil, fmt.Errorf("HTTPBody does not support iteration")
}
func (b *HTTPBody) Index(object.Object) (object.Object, error) {
	return nil, fmt.Errorf("HTTPBody is not indexable")
}
func (b *HTTPBody) GetAttr(name string) (object.Object, error) {
	switch name {
	case "read":
		return &object.Function{Name: "read", Fn: b.Read}, nil
	default:
		return nil, fmt.Errorf("HTTPBody has no attribute '%s'", name)
	}
}

type HTTPResponse struct {
	Status  int
	Headers *object.Dict
	Body    *HTTPBody
}

func NewHTTPResponse(status int, headers *object.Dict, body *HTTPBody) *HTTPResponse {
	return &HTTPResponse{
		Status:  status,
		Headers: headers,
		Body:    body,
	}
}

func (r *HTTPResponse) String() string { return fmt.Sprintf("<http_response %d>", r.Status) }
func (r *HTTPResponse) Bool() bool     { return true }
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
		return r.Body, nil
	default:
		return nil, fmt.Errorf("HTTPResponse has no attribute '%s'", name)
	}
}

func ExecuteHttp() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"request": &object.Function{Name: "request", Fn: httpRequest},
		},
	}, nil
}

func httpRequest(args object.CallArgs) (object.Object, error) {
	method, url, body, headers, err := bindHTTPArgs(args)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := stdhttp.NewRequest(method, url, reader)
	if err != nil {
		return nil, errors.Wrap(err, "request() failed")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := stdhttp.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request() failed")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "request() failed")
	}

	return NewHTTPResponse(resp.StatusCode, headersFromHTTP(resp.Header), NewHTTPBody(data)), nil
}

func bindHTTPArgs(args object.CallArgs) (string, string, string, map[string]string, error) {
	if len(args.Positional) > 4 {
		return "", "", "", nil, fmt.Errorf("request() takes at most 4 positional arguments, got %d", len(args.Positional))
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
			return "", "", "", nil, err
		}
	case 2, 3, 4:
		names := []string{"method", "url", "body", "headers"}
		for i, value := range args.Positional {
			if err := set(names[i], value); err != nil {
				return "", "", "", nil, err
			}
		}
	}

	for key, value := range args.Keyword {
		if key == "header" {
			key = "headers"
		}
		switch key {
		case "method", "url", "body", "headers":
			if err := set(key, value); err != nil {
				return "", "", "", nil, err
			}
		default:
			return "", "", "", nil, fmt.Errorf("request() got an unexpected keyword argument '%s'", key)
		}
	}

	method := "GET"
	if value, ok := values["method"]; ok {
		methodObj, ok := value.(object.String)
		if !ok {
			return "", "", "", nil, fmt.Errorf("request() method argument must be a string, got %T", value)
		}
		method = string(methodObj)
	}

	urlObj, ok := values["url"]
	if !ok {
		return "", "", "", nil, fmt.Errorf("request() missing required argument: 'url'")
	}
	urlString, ok := urlObj.(object.String)
	if !ok {
		return "", "", "", nil, fmt.Errorf("request() url argument must be a string, got %T", urlObj)
	}

	body := ""
	if value, ok := values["body"]; ok {
		switch v := value.(type) {
		case object.Unit:
			body = ""
		case object.String:
			body = string(v)
		default:
			return "", "", "", nil, fmt.Errorf("request() body argument must be a string or nil, got %T", value)
		}
	}

	headers := map[string]string{}
	if value, ok := values["headers"]; ok {
		var err error
		headers, err = headersFromObject(value)
		if err != nil {
			return "", "", "", nil, err
		}
	}

	return method, string(urlString), body, headers, nil
}

func headersFromObject(value object.Object) (map[string]string, error) {
	if _, ok := value.(object.Unit); ok {
		return map[string]string{}, nil
	}

	dict, ok := value.(*object.Dict)
	if !ok {
		return nil, fmt.Errorf("request() headers argument must be a dict or nil, got %T", value)
	}

	headers := make(map[string]string, len(dict.Entries))
	for _, entry := range dict.Entries {
		key, ok := entry.Key.(object.String)
		if !ok {
			return nil, fmt.Errorf("request() header names must be strings, got %T", entry.Key)
		}
		value, ok := entry.Value.(object.String)
		if !ok {
			return nil, fmt.Errorf("request() header values must be strings, got %T", entry.Value)
		}
		headers[string(key)] = string(value)
	}
	return headers, nil
}

func headersFromHTTP(header stdhttp.Header) *object.Dict {
	result := object.NewDict()
	for key, values := range header {
		result.Set(object.String(key), object.String(strings.Join(values, ", ")))
	}
	return result
}

var _ object.Object = (*HTTPBody)(nil)
var _ object.Object = (*HTTPResponse)(nil)
