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
	Req *stdhttp.Request
}

func NewRequest(req *stdhttp.Request) *Request {
	return &Request{objectBase: objectBase{typeName: "Request"}, Req: req}
}

func (r *Request) String() string {
	return fmt.Sprintf("<http_request %s %s>", r.Req.Method, r.Req.URL)
}

func (r *Request) GetAttr(name string) (object.Object, error) {
	switch name {
	case "method":
		return object.String(r.Req.Method), nil
	case "url":
		return object.String(r.Req.URL.String()), nil
	case "header":
		// Shares storage with the request, so header.set/add mutate the
		// request that will be sent by client.do.
		return NewHeader(r.Req.Header), nil
	// TODO: expose "body" once goblin gains a reader/stream type;
	// net/http.Request.Body is an io.ReadCloser with no goblin equivalent yet.
	default:
		return nil, fmt.Errorf("Request has no attribute '%s'", name)
	}
}

var _ object.Object = (*Request)(nil)
