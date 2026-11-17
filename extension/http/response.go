package http

import (
	"fmt"
	"io"
	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/jsonvalue"
	"github.com/aisk/goblin/object"
)

// Response wraps net/http.Response. Its body remains a streaming Body; json()
// consumes that same stream.
type Response struct {
	object.OpaqueBase
	resp *stdhttp.Response
	body *Body
}

func NewResponse(resp *stdhttp.Response) *Response {
	body := NewBody(resp.Body)
	resp.Body = body
	return &Response{OpaqueBase: object.MakeOpaqueBase("Response"), resp: resp, body: body}
}

func (r *Response) String() string {
	return fmt.Sprintf("<http_response %s>", r.resp.Status)
}

func (r *Response) ToString() (string, error) { return r.String(), nil }

func (r *Response) GetAttr(name string) (object.Object, error) {
	switch name {
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
	return []string{"status_code", "status", "header", "body", "json"}
}

// json parses the response body as JSON and returns the corresponding goblin
// value (dict, list, string, number, bool, or nil).
func (r *Response) json(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("json", args).Finish(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(r.body)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "json() failed to read HTTP body", err)
	}
	return jsonvalue.Decode("json", data)
}

var _ object.Object = (*Response)(nil)
