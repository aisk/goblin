package http

import (
	"bytes"
	"errors"
	"io"
	"sync"

	stdhttp "net/http"

	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

// outgoing is the body of one request: fixed bytes that can be sent again
// (String, Bytes, json=, form=), a stream read once, or nothing.
type outgoing struct {
	data        []byte
	hasData     bool
	stream      *recordingReader
	length      int64
	known       bool
	contentType string
}

// makeBody turns body=, json= or form= into what the request sends.
func (r *request) makeBody() (*outgoing, error) {
	switch {
	case r.json != object.Nil:
		data, err := jsonBody(r.fn, r.json)
		if err != nil {
			return nil, err
		}
		return &outgoing{data: data, hasData: true, contentType: "application/json"}, nil
	case r.form != nil:
		query, err := urlvalue.EncodeQuery(r.fn, "form", r.form)
		if err != nil {
			return nil, err
		}
		return &outgoing{data: []byte(query), hasData: true, contentType: "application/x-www-form-urlencoded"}, nil
	case r.body == object.Nil:
		return &outgoing{}, nil
	}
	switch v := r.body.(type) {
	case object.String:
		return &outgoing{data: []byte(v), hasData: true}, nil
	case object.Bytes:
		return &outgoing{data: []byte(v), hasData: true}, nil
	}
	o := &outgoing{}
	if f, ok := fileOf(r.body); ok {
		if mp, ok := f.Source().(*multipartReader); ok && mp.wasStarted() {
			return nil, object.NewValueError("%s() this multipart body was already sent; build it again", r.fn)
		}
		o.length, o.known = f.Remaining()
	}
	reader, ok := object.DataReader(r.fn, "body", r.body)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'body' must be String, Bytes, Nil or have a read(size) method, got %s", r.fn, r.body.TypeName())
	}
	o.stream = &recordingReader{r: reader}
	return o, nil
}

// attach sets the body on req. Fixed bytes can be sent again, so Go follows
// a 307 or 308 with them; a stream cannot, so Go returns that response.
func (o *outgoing) attach(req *stdhttp.Request) {
	switch {
	case o.hasData:
		data := o.data
		req.ContentLength = int64(len(data))
		if len(data) == 0 {
			req.Body = stdhttp.NoBody
			req.GetBody = func() (io.ReadCloser, error) { return stdhttp.NoBody, nil }
			return
		}
		req.Body = io.NopCloser(bytes.NewReader(data))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	case o.stream != nil:
		if o.known && o.length == 0 {
			req.Body = stdhttp.NoBody
			return
		}
		req.Body = io.NopCloser(o.stream)
		req.ContentLength = -1
		if o.known {
			req.ContentLength = o.length
		}
	}
}

// finish stops the stream from being read any further and returns the
// Goblin error its read raised, if any.
func (o *outgoing) finish() error {
	if o.stream == nil {
		return nil
	}
	return o.stream.finish()
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

func (rr *recordingReader) finish() error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.done = true
	return rr.raised
}
