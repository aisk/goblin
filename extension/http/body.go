package http

import (
	"bytes"
	"errors"
	"io"
	"sync"

	stdhttp "net/http"

	"github.com/aisk/goblin/object"
)

// claim checks, before anything is sent, that the request's body can be
// sent now, and marks a stream body as sent: a stream is good for one
// exchange.
func (r *Request) claim(fn string) error {
	c := r.content
	if c.fixed {
		return nil
	}
	if c.sent.Load() {
		return object.NewValueError("%s() the body of this request is a stream that was already sent; build the request again", fn)
	}
	if mp, ok := c.body.Source().(*multipartReader); ok && mp.wasStarted() {
		return object.NewValueError("%s() this multipart body was already sent; build it again", fn)
	}
	if c.sent.Swap(true) {
		return object.NewValueError("%s() the body of this request is a stream that was already sent; build the request again", fn)
	}
	return nil
}

// attach sets the request's body on req and returns the reader that feeds
// a stream body, nil for fixed bytes. Fixed bytes can be sent again, so Go
// follows a 307 or 308 with them; a stream cannot, so Go returns that
// response.
func (r *Request) attach(fn string, req *stdhttp.Request) *recordingReader {
	c := r.content
	if c.fixed {
		data := c.data
		req.ContentLength = int64(len(data))
		if len(data) == 0 {
			req.Body = stdhttp.NoBody
			req.GetBody = func() (io.ReadCloser, error) { return stdhttp.NoBody, nil }
			return nil
		}
		req.Body = io.NopCloser(bytes.NewReader(data))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
		return nil
	}
	length, known := c.body.Remaining()
	// The File is read through its read method, as any readable is, so
	// what line reading has buffered is sent first.
	reader, _ := object.DataReader(fn, "body", c.body)
	rr := &recordingReader{r: reader}
	if known && length == 0 {
		req.Body = stdhttp.NoBody
		return rr
	}
	req.Body = io.NopCloser(rr)
	req.ContentLength = -1
	if known {
		req.ContentLength = length
	}
	return rr
}

// recordingReader reads a caller's stream for the transport. It keeps the
// first Goblin error the stream raised, so that a failed request reports
// it unchanged, and once the request has returned it refuses further reads,
// so the transport never calls into Goblin code behind the program's back.
type recordingReader struct {
	r      io.Reader
	mu     sync.Mutex
	done   bool
	raised error
}

var errAbandoned = errors.New("the request has already returned")

func (rr *recordingReader) Read(p []byte) (int, error) {
	rr.mu.Lock()
	if rr.done {
		rr.mu.Unlock()
		return 0, errAbandoned
	}
	rr.mu.Unlock()
	n, err := rr.r.Read(p)
	if err != nil && err != io.EOF {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			rr.mu.Lock()
			if rr.raised == nil {
				rr.raised = err
			}
			rr.mu.Unlock()
		}
	}
	return n, err
}

// finish stops the stream from being read any further and returns the
// Goblin error its read raised, if any.
func (rr *recordingReader) finish() error {
	if rr == nil {
		return nil
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.done = true
	return rr.raised
}
