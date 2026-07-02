package http

import (
	"fmt"
	stdhttp "net/http"

	"github.com/aisk/goblin/object"
)

// Client wraps net/http.Client, mirroring its Do/Get/Head/Post methods (plus
// put/patch/delete for convenience). Reusing a single client across requests
// reuses connections. The Timeout is set at construction via
// http.Client(timeout=...).
type Client struct {
	objectBase
	Client *stdhttp.Client
}

func NewClient(c *stdhttp.Client) *Client {
	return &Client{objectBase: objectBase{typeName: "Client"}, Client: c}
}

func (c *Client) String() string {
	return "<http_client>"
}

func (c *Client) GetAttr(name string) (object.Object, error) {
	switch name {
	case "timeout":
		return object.Float(c.Client.Timeout.Seconds()), nil
	case "do":
		return &object.Function{Name: "do", Fn: func(args object.CallArgs) (object.Object, error) {
			return doDo(c.Client, args)
		}}, nil
	case "get":
		return &object.Function{Name: "get", Fn: func(args object.CallArgs) (object.Object, error) {
			return doGet(c.Client, args)
		}}, nil
	case "head":
		return &object.Function{Name: "head", Fn: func(args object.CallArgs) (object.Object, error) {
			return doHead(c.Client, args)
		}}, nil
	case "delete":
		return &object.Function{Name: "delete", Fn: func(args object.CallArgs) (object.Object, error) {
			return doDelete(c.Client, args)
		}}, nil
	case "post":
		return &object.Function{Name: "post", Fn: func(args object.CallArgs) (object.Object, error) {
			return doPost(c.Client, args)
		}}, nil
	case "put":
		return &object.Function{Name: "put", Fn: func(args object.CallArgs) (object.Object, error) {
			return doPut(c.Client, args)
		}}, nil
	case "patch":
		return &object.Function{Name: "patch", Fn: func(args object.CallArgs) (object.Object, error) {
			return doPatch(c.Client, args)
		}}, nil
	default:
		return nil, fmt.Errorf("Client has no attribute '%s'", name)
	}
}

var _ object.Object = (*Client)(nil)
