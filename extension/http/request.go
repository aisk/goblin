package http

import (
	"fmt"
	stdhttp "net/http"

	"github.com/aisk/goblin/object"
)

// Request wraps net/http.Request. It is produced by the http.Request
// constructor and consumed by client.do, mirroring Go's http.NewRequest /
// (*Client).Do flow.
type Request struct {
	objectBase
	Req  *stdhttp.Request
	body *Body
}

func NewRequest(req *stdhttp.Request) *Request {
	r := &Request{objectBase: objectBase{typeName: "Request"}, Req: req}
	if req.Body != nil {
		r.body = NewBody(req.Body)
		req.Body = r.body
	}
	return r
}

func (r *Request) String() string {
	return fmt.Sprintf("<http_request %s %s>", r.Req.Method, r.Req.URL)
}

func (r *Request) ToString() (string, error) { return r.String(), nil }

func (r *Request) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(r), nil
	case "method":
		return object.String(r.Req.Method), nil
	case "url":
		return object.String(r.Req.URL.String()), nil
	case "header":
		// Shares storage with the request, so header.set/add mutate the
		// request that will be sent by client.do.
		return NewHeader(r.Req.Header), nil
	case "body":
		if r.body == nil {
			return object.Nil, nil
		}
		return r.body, nil
	default:
		return nil, object.NewAttributeError("Request has no attribute '%s'", name)
	}
}

func (r *Request) Attributes() []string {
	return []string{"attributes", "method", "url", "header", "body"}
}

var _ object.Object = (*Request)(nil)
