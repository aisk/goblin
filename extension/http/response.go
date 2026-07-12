package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhttp "net/http"

	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
)

// Response wraps net/http.Response. It exposes the Go field names (status_code,
// status, header) plus a json() helper that decodes the buffered body. The raw
// body is read eagerly and kept in memory.
type Response struct {
	objectBase
	resp *stdhttp.Response
	body []byte
}

func NewResponse(resp *stdhttp.Response, body []byte) *Response {
	return &Response{objectBase: objectBase{typeName: "Response"}, resp: resp, body: body}
}

func (r *Response) String() string {
	return fmt.Sprintf("<http_response %s>", r.resp.Status)
}

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
	case "json":
		return &object.Function{Name: "json", Fn: r.json}, nil
	// TODO: expose "body" once goblin gains a reader/bytes type; the raw
	// response bytes are intentionally not surfaced yet. Until then, json()
	// is the way to read the response payload.
	default:
		return nil, object.NewAttributeError("Response has no attribute '%s'", name)
	}
}

func (r *Response) Attributes() []string {
	return []string{"attributes", "status_code", "status", "header", "json"}
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
	dec := json.NewDecoder(bytes.NewReader(r.body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, object.WrapError(object.ParseError, "json() failed to parse response body", err)
	}
	return extension.JSONToGoblin(v)
}

var _ object.Object = (*Response)(nil)
