package http

import (
	"net/http/cookiejar"
	"time"

	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

var clientType = object.NewNativeConstructor("Client", newClient)

// Client holds what several requests share: the connection setting
// (insecure), default headers, the default timeout, a cookie store and the
// redirect limit. It does not hold a base URL.
type Client struct {
	object.OpaqueBase
	headers   *Headers
	timeout   time.Duration
	limited   bool
	jar       *cookiejar.Jar
	insecure  bool
	redirects int
}

var _ object.Object = (*Client)(nil)

// defaultClient is Client() with every default: what each module function
// runs on, a new one per call.
func defaultClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{OpaqueBase: object.MakeOpaqueBase("Client"), headers: emptyHeaders(), timeout: defaultTimeout, limited: true, jar: jar, redirects: defaultRedirects}
}

// newClient is Client(headers=nil, timeout=30, cookies=nil, insecure=false,
// redirects=10).
func newClient(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Client", args)
	headersObj := p.AnyOr("headers", object.Nil)
	timeout, limited, given := p.OptionalTimeout("timeout")
	cookiesObj := p.AnyOr("cookies", object.Nil)
	insecure := bool(p.BoolOr("insecure", false))
	redirects := p.IntOr("redirects", defaultRedirects)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if redirects < 0 {
		return nil, object.NewValueError("Client() argument 'redirects' must be non-negative, got %d", int64(redirects))
	}
	c := defaultClient()
	headers, err := headersFrom("Client", "headers", headersObj)
	if err != nil {
		return nil, err
	}
	c.headers = headers
	if given {
		c.timeout, c.limited = timeout, limited
	}
	if err := startCookies(c.jar, cookiesObj); err != nil {
		return nil, err
	}
	c.insecure = insecure
	c.redirects = int(redirects)
	return c, nil
}

// startCookies puts Client(cookies={url: {name: value}}) in the jar: each
// cookie for the host of its URL, every path, only over TLS for https, kept
// for the life of the Client.
func startCookies(jar *cookiejar.Jar, v object.Object) error {
	if v == object.Nil {
		return nil
	}
	d, ok := v.(*object.Dict)
	if !ok {
		return object.NewTypeError("Client() argument 'cookies' must be Dict or Nil, got %s", v.TypeName())
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
	for _, cookie := range c.jar.Cookies(u) {
		d.Set(object.String(cookie.Name), object.String(cookie.Value))
	}
	return d, nil
}

// callTimeout reads timeout= with the client's value as the default; nil
// means no limit.
func (c *Client) callTimeout(p *object.ArgParser) (time.Duration, bool) {
	timeout, limited, given := p.OptionalTimeout("timeout")
	if !given {
		return c.timeout, c.limited
	}
	return timeout, limited
}

// exchangeCall is exchange(request, timeout=<client's>).
func (c *Client) exchangeCall(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("exchange", args)
	reqObj := p.Any("request")
	timeout, limited := c.callTimeout(p)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	req, err := asRequest("exchange", reqObj)
	if err != nil {
		return nil, err
	}
	return c.run("exchange", req, false, timeout, limited, false)
}

// sendCall is send(request, timeout=<client's>, check=true).
func (c *Client) sendCall(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("send", args)
	reqObj := p.Any("request")
	timeout, limited := c.callTimeout(p)
	check := bool(p.BoolOr("check", true))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	req, err := asRequest("send", reqObj)
	if err != nil {
		return nil, err
	}
	return c.run("send", req, true, timeout, limited, check)
}

// requestCall is request(method, url, headers=nil, body=nil,
// timeout=<client's>, check=true): send of the Request the arguments build.
func (c *Client) requestCall(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("request", args)
	method := p.Str("method")
	target := p.Any("url")
	headers := p.AnyOr("headers", object.Nil)
	body := p.AnyOr("body", object.Nil)
	timeout, limited := c.callTimeout(p)
	check := bool(p.BoolOr("check", true))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	req, err := buildRequest("request", string(method), target, headers, body)
	if err != nil {
		return nil, err
	}
	return c.run("request", req, true, timeout, limited, check)
}

// getCall is get(url, headers=nil, timeout=<client's>, check=true):
// request("GET", url, ...).
func (c *Client) getCall(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("get", args)
	target := p.Any("url")
	headers := p.AnyOr("headers", object.Nil)
	timeout, limited := c.callTimeout(p)
	check := bool(p.BoolOr("check", true))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	req, err := buildRequest("get", "GET", target, headers, object.Nil)
	if err != nil {
		return nil, err
	}
	return c.run("get", req, true, timeout, limited, check)
}

func asRequest(fn string, v object.Object) (*Request, error) {
	r, ok := v.(*Request)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'request' must be Request, got %s", fn, v.TypeName())
	}
	return r, nil
}

func (c *Client) GetAttr(name string) (object.Object, error) {
	switch name {
	case "exchange":
		return &object.Function{Name: "exchange", Fn: c.exchangeCall}, nil
	case "send":
		return &object.Function{Name: "send", Fn: c.sendCall}, nil
	case "request":
		return &object.Function{Name: "request", Fn: c.requestCall}, nil
	case "get":
		return &object.Function{Name: "get", Fn: c.getCall}, nil
	case "cookies":
		return &object.Function{Name: "cookies", Fn: c.cookies}, nil
	}
	return nil, object.NewAttributeError("Client has no attribute '%s'", name)
}

func (c *Client) Attributes() []string {
	return []string{"exchange", "send", "request", "get", "cookies"}
}
