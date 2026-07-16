package http

import (
	"encoding/json"
	"fmt"
	stdhttp "net/http"

	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
)

// Response wraps net/http.Response. Its body remains a streaming Body; json()
// consumes that same stream.
type Response struct {
	objectBase
	resp *stdhttp.Response
	body *Body
}

func NewResponse(resp *stdhttp.Response) *Response {
	body := NewBody(resp.Body)
	resp.Body = body
	return &Response{objectBase: objectBase{typeName: "Response"}, resp: resp, body: body}
}

func (r *Response) String() string {
	return fmt.Sprintf("<http_response %s>", r.resp.Status)
}

func (r *Response) ToString() (string, error) { return r.String(), nil }

func (r *Response) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(r), nil
	case "status_code":
		return object.Integer(r.resp.StatusCode), nil
	case "status":
		return object.String(r.resp.Status), nil
	case "header":
		return NewHeader(r.resp.Header), nil
	case "body":
		return r.body, nil
	case "json":
		return &object.Function{Name: "json", Fn: r.json}, nil
	default:
		return nil, object.NewAttributeError("Response has no attribute '%s'", name)
	}
}

func (r *Response) Attributes() []string {
	return []string{"attributes", "status_code", "status", "header", "body", "json"}
}

// json parses the response body as JSON and returns the corresponding goblin
// value (dict, list, string, number, bool, or nil).
func (r *Response) json(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("json", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("json() takes exactly 0 arguments, got %d", len(args.Positional))
	}
	dec := json.NewDecoder(r.body)
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, object.WrapError(object.ParseError, "json() failed to parse response body", err)
	}
	return extension.JSONToGoblin(v)
}

var _ object.Object = (*Response)(nil)
