package http

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/urlvalue"
	"github.com/aisk/goblin/object"
)

// multipartBody is http.multipart(fields=nil, files=nil): a multipart/form-data
// body as a read-only File, produced as it is read, and its Content-Type.
// It sends nothing and reads nothing now.
func multipartBody(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("multipart", args)
	fieldsObj := p.AnyOr("fields", object.Nil)
	filesObj := p.AnyOr("files", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	fields, err := optionalDict("multipart", "fields", fieldsObj)
	if err != nil {
		return nil, err
	}
	files, err := optionalDict("multipart", "files", filesObj)
	if err != nil {
		return nil, err
	}

	var head bytes.Buffer
	w := multipart.NewWriter(&head)
	mr := &multipartReader{}
	// Each part's delimiter and headers become a fixed segment, cut from
	// what the writer has produced; the content follows as its own segment.
	cut := func() []byte {
		b := append([]byte(nil), head.Bytes()...)
		head.Reset()
		return b
	}
	if fields != nil {
		var failed error
		err := urlvalue.Fields("multipart", "fields", fields, func(name, value string) {
			if failed != nil {
				return
			}
			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", `form-data; name="`+escapeQuotes(name)+`"`)
			if _, err := w.CreatePart(h); err != nil {
				failed = err
				return
			}
			mr.segments = append(mr.segments, segment{fixed: cut()}, segment{fixed: []byte(value)})
		})
		if err == nil {
			err = failed
		}
		if err != nil {
			return nil, err
		}
	}
	if files != nil {
		for _, entry := range files.Entries() {
			field, ok := entry.Key.(object.String)
			if !ok {
				return nil, object.NewTypeError("multipart() argument 'files' keys must be String, got %s", entry.Key.TypeName())
			}
			parts := []object.Object{entry.Value}
			if list, ok := entry.Value.(*object.List); ok {
				parts = list.Elements
			}
			for _, value := range parts {
				part, err := filePart(string(field), value)
				if err != nil {
					return nil, err
				}
				h := textproto.MIMEHeader{}
				h.Set("Content-Disposition", `form-data; name="`+escapeQuotes(string(field))+`"; filename="`+escapeQuotes(part.name)+`"`)
				h.Set("Content-Type", part.contentType)
				if _, err := w.CreatePart(h); err != nil {
					return nil, object.WrapError(object.ValueError, "multipart() cannot encode the form", err)
				}
				mr.segments = append(mr.segments, segment{fixed: cut()}, segment{data: part.data})
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, object.WrapError(object.ValueError, "multipart() cannot encode the form", err)
	}
	mr.segments = append(mr.segments, segment{fixed: cut()})
	body := filestream.NewReader("multipart", mr, nil)
	return &object.List{Elements: []object.Object{body, object.String(w.FormDataContentType())}}, nil
}

func optionalDict(fn, param string, v object.Object) (*object.Dict, error) {
	if v == object.Nil {
		return nil, nil
	}
	d, ok := v.(*object.Dict)
	if !ok {
		return nil, object.NewTypeError("%s() argument '%s' must be Dict or Nil, got %s", fn, param, v.TypeName())
	}
	return d, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string { return quoteEscaper.Replace(s) }

// part is one file of a multipart body.
type part struct {
	name        string
	contentType string
	data        object.Object
}

// filePart reads one value of files=: an open File that has a path, or a
// Dict with "data" and, unless data is such a File, "name".
func filePart(field string, v object.Object) (*part, error) {
	needsName := func() error {
		return object.NewValueError(`multipart() file "%s" needs a name: pass an open fs.File, or {"name": …, "data": …}`, field)
	}
	switch value := v.(type) {
	case *filestream.File:
		name, ok := fileName(value)
		if !ok {
			return nil, needsName()
		}
		return &part{name: name, contentType: "application/octet-stream", data: value}, nil
	case *object.Dict:
		p := &part{contentType: "application/octet-stream"}
		hasName := false
		for _, entry := range value.Entries() {
			key, _ := entry.Key.(object.String)
			switch key {
			case "data":
				p.data = entry.Value
			case "name":
				name, ok := entry.Value.(object.String)
				if !ok {
					return nil, object.NewTypeError(`multipart() file "%s" name must be String, got %s`, field, entry.Value.TypeName())
				}
				p.name, hasName = string(name), true
			case "content_type":
				ct, ok := entry.Value.(object.String)
				if !ok {
					return nil, object.NewTypeError(`multipart() file "%s" content_type must be String, got %s`, field, entry.Value.TypeName())
				}
				p.contentType = string(ct)
			default:
				text, _ := entry.Key.ToString()
				return nil, object.NewValueError(`multipart() file "%s" has an unknown key "%s"; a part takes "data", "name" and "content_type"`, field, text)
			}
		}
		if p.data == nil {
			return nil, object.NewValueError(`multipart() file "%s" needs "data"`, field)
		}
		if !isData(p.data) {
			return nil, object.NewTypeError(`multipart() file "%s" data must be String, Bytes or have a read(size) method, got %s`, field, p.data.TypeName())
		}
		if !hasName {
			f, ok := p.data.(*filestream.File)
			if !ok {
				return nil, needsName()
			}
			if p.name, ok = fileName(f); !ok {
				return nil, needsName()
			}
		}
		return p, nil
	}
	if isData(v) {
		return nil, needsName()
	}
	return nil, object.NewTypeError("multipart() argument 'files' values must be File, Dict or List, got %s", v.TypeName())
}

func isData(v object.Object) bool {
	switch v.(type) {
	case object.String, object.Bytes:
		return true
	}
	_, ok := object.DataReader("multipart", "files", v)
	return ok
}

// fileName is the name of the file a File was opened from.
func fileName(f *filestream.File) (string, bool) {
	path, err := f.GetAttr("path")
	if err != nil || path == object.Nil {
		return "", false
	}
	name, err := path.GetAttr("name")
	if err != nil {
		return "", false
	}
	s, ok := name.(object.String)
	return string(s), ok && s != ""
}

// segment is a piece of the encoded body: fixed bytes, or a part's data,
// read when the body is.
type segment struct {
	fixed []byte
	data  object.Object
}

func (s segment) size() (int64, bool) {
	if s.data == nil {
		return int64(len(s.fixed)), true
	}
	switch v := s.data.(type) {
	case object.String:
		return int64(len(v)), true
	case object.Bytes:
		return int64(len(v)), true
	case *filestream.File:
		return v.Remaining()
	}
	return 0, false
}

func (s segment) reader() io.Reader {
	if s.data == nil {
		return bytes.NewReader(s.fixed)
	}
	r, _ := object.DataReader("multipart", "files", s.data)
	return r
}

// multipartReader produces the body from its segments. It remembers that
// reading started, because a body sent once cannot be sent again.
type multipartReader struct {
	mu       sync.Mutex
	segments []segment
	r        io.Reader
	started  bool
}

func (m *multipartReader) Read(p []byte) (int, error) {
	m.mu.Lock()
	if !m.started {
		m.started = true
		readers := make([]io.Reader, len(m.segments))
		for i, s := range m.segments {
			readers[i] = s.reader()
		}
		m.r = io.MultiReader(readers...)
	}
	r := m.r
	m.mu.Unlock()
	return r.Read(p)
}

func (m *multipartReader) wasStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// Remaining is the length of the whole body when every part's length is
// known without reading it; it is measured when the request starts.
func (m *multipartReader) Remaining() (int64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return 0, false
	}
	total := int64(0)
	for _, s := range m.segments {
		n, ok := s.size()
		if !ok {
			return 0, false
		}
		total += n
	}
	return total, true
}
