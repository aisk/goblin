// Package http is the http module, an HTTP client built on two messages,
// Request and Response. exchange performs one request and returns one
// response, adding and interpreting nothing; send is a user agent's request,
// defined as steps over exchange (default headers, User-Agent, gzip,
// cookies, redirects, the status check); request and get build the Request
// and call send.
package http

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	stdhttp "net/http"

	"github.com/aisk/goblin/object"
)

// StatusError is raised by send under check=true for a status of 400 or
// above, or a redirect that send was told to follow and did not. Data:
// status, response.
var StatusError = object.NewErrorKind("StatusError", object.BaseError)

// userAgent is what send adds when neither the request nor the Client's
// headers name a User-Agent.
const userAgent = "goblin/" + object.Version

const (
	defaultTimeout   = 30 * time.Second
	defaultRedirects = 10
)

func Execute() (object.Object, error) {
	return object.NewModule("http", map[string]object.Object{
		"Headers":     headersType.Function,
		"Request":     requestType.Function,
		"Response":    responseType.Function,
		"Client":      clientType.Function,
		"StatusError": StatusError,
	}, object.Funcs{
		"exchange":  func(args object.CallArgs) (object.Object, error) { return defaultClient().exchangeCall(args) },
		"send":      func(args object.CallArgs) (object.Object, error) { return defaultClient().sendCall(args) },
		"request":   func(args object.CallArgs) (object.Object, error) { return defaultClient().requestCall(args) },
		"get":       func(args object.CallArgs) (object.Object, error) { return defaultClient().getCall(args) },
		"multipart": multipartBody,
	}), nil
}

// The two shared transports are clones of Go's default one (so they keep
// HTTP/2, the proxy from the environment and connection reuse) with Go's
// own gzip handling off: send does its own, and exchange does none. The
// insecure one accepts any certificate and is made on first use; its pool is
// separate, so a connection opened without verification is never reused by
// a verifying Client.
var (
	quietLog          sync.Once
	transportOnce     sync.Once
	secureTransport   *stdhttp.Transport
	insecureOnce      sync.Once
	insecureTransport *stdhttp.Transport
)

func transport(insecure bool) *stdhttp.Transport {
	// Go's transport and cookie code report what they repair or drop (an
	// unsolicited response, an invalid cookie byte) through the standard
	// logger, which writes to stderr. That output is Go's machinery, not
	// the program's, so it is discarded before the first request.
	quietLog.Do(func() { log.SetOutput(io.Discard) })
	transportOnce.Do(func() {
		t := stdhttp.DefaultTransport.(*stdhttp.Transport).Clone()
		t.DisableCompression = true
		secureTransport = t
	})
	if !insecure {
		return secureTransport
	}
	insecureOnce.Do(func() {
		t := stdhttp.DefaultTransport.(*stdhttp.Transport).Clone()
		t.DisableCompression = true
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		insecureTransport = t
	})
	return insecureTransport
}

// watchdog bounds a call, and after it each wait for body data, by
// cancelling the exchange's context when the limit passes.
type watchdog struct {
	d       time.Duration
	limited bool
	cancel  context.CancelFunc

	mu    sync.Mutex
	gen   int
	armed bool
	timer *time.Timer
	fired atomic.Bool
}

func (w *watchdog) arm() {
	if !w.limited {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.gen++
	gen := w.gen
	w.armed = true
	w.timer = time.AfterFunc(w.d, func() {
		w.mu.Lock()
		if !w.armed || w.gen != gen {
			w.mu.Unlock()
			return
		}
		w.fired.Store(true)
		w.mu.Unlock()
		w.cancel()
	})
}

// disarm stops the timer and reports whether the limit has not passed.
func (w *watchdog) disarm() bool {
	if !w.limited {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.armed = false
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	return !w.fired.Load()
}

// framing names the header fields that describe one connection or one wire
// format. The library computes them; the request's own are not sent.
var framing = []string{"Content-Length", "Transfer-Encoding", "Connection", "Keep-Alive", "Proxy-Connection", "Te", "Upgrade"}

// run performs exchange (agent false) or send (agent true) of req, bounded
// by timeout when limited. fn names the function the user called.
func (c *Client) run(fn string, req *Request, agent bool, timeout time.Duration, limited bool, check bool) (object.Object, error) {
	if req.method == "CONNECT" {
		return nil, object.NewValueError("%s() cannot send a CONNECT request", fn)
	}
	if err := req.claim(fn); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	dog := &watchdog{d: timeout, limited: limited, cancel: cancel}
	target := req.url.String()
	goReq, err := stdhttp.NewRequestWithContext(ctx, req.method, target, nil)
	if err != nil {
		cancel()
		return nil, object.WrapError(object.ValueError, fn+"() cannot build the request", err)
	}
	header := req.headers.toGo()
	for _, name := range framing {
		delete(header, name)
	}
	if host, ok := header["Host"]; ok {
		if len(host) > 0 {
			goReq.Host = host[0]
		}
		delete(header, "Host")
	}
	gzipAsked := false
	if agent {
		for _, f := range c.headers.fields {
			if !req.headers.has(f.name) {
				key := strings.ToLower(f.name)
				if key == "host" {
					if len(f.values) > 0 {
						goReq.Host = f.values[0]
					}
					continue
				}
				if isFraming(key) {
					continue
				}
				header[canonical(f.name)] = append([]string(nil), f.values...)
			}
		}
		if _, ok := header["User-Agent"]; !ok {
			header["User-Agent"] = []string{userAgent}
		}
		if _, ok := header["Accept-Encoding"]; !ok && req.method != "HEAD" {
			if _, ranged := header["Range"]; !ranged {
				header["Accept-Encoding"] = []string{"gzip"}
				gzipAsked = true
			}
		}
	} else if _, ok := header["User-Agent"]; !ok {
		// An empty User-Agent makes Go send none, on HTTP/1.1 and HTTP/2.
		header["User-Agent"] = []string{""}
	}
	goReq.Header = header
	stream := req.attach(fn, goReq)

	client := &stdhttp.Client{Transport: transport(c.insecure)}
	limitReached := false
	if agent {
		client.Jar = c.jar
		referer, hadReferer := header["Referer"]
		client.CheckRedirect = func(next *stdhttp.Request, via []*stdhttp.Request) error {
			if c.redirects == 0 {
				return stdhttp.ErrUseLastResponse
			}
			if len(via) > c.redirects {
				limitReached = true
				return stdhttp.ErrUseLastResponse
			}
			// Go adds the previous URL as a Referer; send adds none, and a
			// caller's own is sent as it was given.
			if hadReferer {
				next.Header["Referer"] = referer
			} else {
				delete(next.Header, "Referer")
			}
			return nil
		}
	} else {
		client.CheckRedirect = func(*stdhttp.Request, []*stdhttp.Request) error { return stdhttp.ErrUseLastResponse }
	}

	dog.arm()
	resp, err := client.Do(goReq)
	// From here on nothing reads the caller's stream: a transport that is
	// still writing gets an error instead of calling into Goblin code.
	raised := stream.finish()
	if err != nil {
		fired := !dog.disarm()
		cancel()
		if raised != nil {
			return nil, raised
		}
		if fired {
			return nil, timeoutError(fn, timeout)
		}
		return nil, exchangeError(fn, req.url, err)
	}

	gunzip := gzipAsked && strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip")
	response, body := receivedResponse(resp, gunzip, dog, cancel)

	if agent && check {
		unfollowed := c.redirects > 0 && isRedirect(resp.StatusCode) && resp.Header.Get("Location") != ""
		if resp.StatusCode >= 400 || unfollowed {
			data := body.readAll()
			if !dog.disarm() {
				return nil, timeoutError(fn, timeout)
			}
			response.body.Close(object.CallArgs{})
			buffered, _ := makeContent(fn, object.Bytes(data))
			response.body = buffered.body
			var e *object.Error
			switch {
			case resp.StatusCode >= 400:
				e = object.NewKindError(StatusError, "%s() %s returned %s", fn, target, statusPhrase(resp.StatusCode))
			case limitReached:
				e = object.NewKindError(StatusError, "%s() %s was redirected more than %d times", fn, target, c.redirects)
			default:
				e = object.NewKindError(StatusError, "%s() %s answered %d and the body is a stream that cannot be sent again", fn, target, resp.StatusCode)
			}
			return nil, e.With("status", object.Integer(resp.StatusCode)).With("response", response)
		}
	}
	if !dog.disarm() {
		body.Close()
		return nil, timeoutError(fn, timeout)
	}
	return response, nil
}

func itoa(n int) string { return strconv.Itoa(n) }

func isFraming(lower string) bool {
	for _, name := range framing {
		if strings.EqualFold(name, lower) {
			return true
		}
	}
	return false
}

func canonical(name string) string { return stdhttp.CanonicalHeaderKey(name) }

func isRedirect(status int) bool {
	switch status {
	case 301, 302, 303, 307, 308:
		return true
	}
	return false
}

// statusPhrase is the number and the standard phrase for it, or the number
// alone for a status without one.
func statusPhrase(status int) string {
	text := stdhttp.StatusText(status)
	if text == "" {
		return itoa(status)
	}
	return itoa(status) + " " + text
}

// exchangeError reports a request that got no response.
func exchangeError(fn string, u *url.URL, err error) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) {
		return err
	}
	cause := err
	var ue *url.Error
	if errors.As(err, &ue) {
		cause = ue.Err
	}
	// Go reports a body that turned out shorter or longer than the length
	// measured when the request started only in its message.
	if strings.Contains(cause.Error(), "ContentLength=") {
		return object.WrapError(object.NetworkError, fn+"() the body changed size while it was sent to "+u.Host, cause)
	}
	return object.WrapError(object.NetworkError, fn+"() cannot reach "+u.Host, cause)
}

func timeoutError(fn string, timeout time.Duration) error {
	return object.NewTimeoutError("%s() timed out after %s seconds", fn, seconds(timeout))
}
