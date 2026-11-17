package http

import (
	"net/http/cookiejar"

	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

// Client holds what several requests share: default headers and timeout, a
// session of cookies, whether redirects are followed, and the TLS setting.
// It does not hold a base URL.
type Client struct {
	object.OpaqueBase
	s *session
}

var _ object.Object = (*Client)(nil)

// newClient is Client(headers=nil, timeout=30, cookies=nil, insecure=false,
// redirects=true).
func newClient(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Client", args)
	headersObj := p.AnyOr("headers", object.Nil)
	timeout, limited, given := p.OptionalTimeout("timeout")
	cookiesObj := p.AnyOr("cookies", object.Nil)
	insecure := bool(p.BoolOr("insecure", false))
	redirects := bool(p.BoolOr("redirects", true))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	headers, err := headerDict("Client", "headers", headersObj)
	if err != nil {
		return nil, err
	}
	if !given {
		timeout, limited = defaultTimeout, true
	}
	jar, _ := cookiejar.New(nil)
	if err := startCookies(jar, cookiesObj); err != nil {
		return nil, err
	}
	return &Client{OpaqueBase: object.MakeOpaqueBase("Client"), s: &session{
		headers:   headers,
		timeout:   timeout,
		limited:   limited,
		jar:       jar,
		insecure:  insecure,
		redirects: redirects,
	}}, nil
}

// startCookies puts Client(cookies={url: {name: value}}) in the jar: each
// cookie for the host of its URL, every path, only over TLS for https, kept
// for the life of the Client.
func startCookies(jar *cookiejar.Jar, v object.Object) error {
	d, err := optionalDict("Client", "cookies", v)
	if err != nil || d == nil {
		return err
	}
	for _, entry := range d.Entries() {
		switch entry.Key.(type) {
		case object.String, *urlvalue.URL:
		default:
			return object.NewTypeError("Client() argument 'cookies' keys must be String or URL, got %s", entry.Key.TypeName())
		}
		u, err := urlvalue.FromObject("Client", "cookies", entry.Key)
		if err != nil {
			return err
		}
		values, ok := entry.Value.(*object.Dict)
		if !ok {
			return object.NewTypeError("Client() argument 'cookies' values must be Dict, got %s", entry.Value.TypeName())
		}
		var cookies []*stdhttp.Cookie
		for _, c := range values.Entries() {
			name, ok1 := c.Key.(object.String)
			value, ok2 := c.Value.(object.String)
			if !ok1 || !ok2 {
				return object.NewTypeError("Client() argument 'cookies' must map each URL to a Dict of String to String")
			}
			cookies = append(cookies, &stdhttp.Cookie{Name: string(name), Value: string(value), Path: "/", Secure: u.Scheme == "https"})
		}
		jar.SetCookies(u, cookies)
	}
	return nil
}

func (c *Client) Constructor() *object.Function { return clientType.Function }
func (c *Client) String() string                { return "<Client>" }
func (c *Client) ToString() (string, error)     { return c.String(), nil }

// cookies is Client.cookies(url): the cookies the client would send to url
// now, as a new Dict of name to value.
func (c *Client) cookies(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("cookies", args)
	target := p.Any("url")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	u, err := urlvalue.FromObject("cookies", "url", target)
	if err != nil {
		return nil, err
	}
	d := object.NewDict()
	for _, cookie := range c.s.jar.Cookies(u) {
		d.Set(object.String(cookie.Name), object.String(cookie.Value))
	}
	return d, nil
}

func (c *Client) method(fn, method string, withBody bool) *object.Function {
	return &object.Function{Name: fn, Fn: func(args object.CallArgs) (object.Object, error) {
		return c.s.call(fn, method, withBody, args)
	}}
}

func (c *Client) GetAttr(name string) (object.Object, error) {
	switch name {
	case "get":
		return c.method("get", "GET", false), nil
	case "delete":
		return c.method("delete", "DELETE", false), nil
	case "post":
		return c.method("post", "POST", true), nil
	case "put":
		return c.method("put", "PUT", true), nil
	case "patch":
		return c.method("patch", "PATCH", true), nil
	case "request":
		return c.method("request", "", true), nil
	case "cookies":
		return &object.Function{Name: "cookies", Fn: c.cookies}, nil
	}
	return nil, object.NewAttributeError("Client has no attribute '%s'", name)
}

func (c *Client) Attributes() []string {
	return []string{"get", "delete", "post", "put", "patch", "request", "cookies"}
}

