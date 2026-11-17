package object

import (
	"bytes"
	"io"
)

// PathValue is implemented by a native value that names a file (the fs
// module's Path). It lets the runtime accept such a value wherever a path is
// expected without importing the module that defines it.
type PathValue interface {
	Object
	FilePath() string
}

// PathString returns the file path a String or a PathValue names.
func PathString(obj Object) (string, bool) {
	switch v := obj.(type) {
	case String:
		return string(v), true
	case PathValue:
		return v.FilePath(), true
	}
	return "", false
}

// Path returns a required argument that names a file, a String or a Path,
// as the path text.
func (p *ArgParser) Path(name string) string {
	v, ok := p.required(name)
	if !ok {
		return ""
	}
	s, ok := PathString(v)
	if !ok {
		p.typeErr(name, "String or Path", v)
	}
	return s
}

// NullablePath returns an optional path argument and whether there is one:
// leaving it out and passing nil both report false.
func (p *ArgParser) NullablePath(name string) (string, bool) {
	v, ok := p.next(name)
	if !ok || v == Nil {
		return "", false
	}
	s, ok := PathString(v)
	if !ok {
		p.typeErr(name, "String, Path or Nil", v)
	}
	return s, ok
}

// DataReader returns a reader over v as a `data` argument accepts it: the
// UTF-8 bytes of a String, the bytes of Bytes, or the stream of anything with
// a read(size) method, which is read from but never closed. ok is false for
// any other value.
func DataReader(fn, param string, v Object) (io.Reader, bool) {
	switch d := v.(type) {
	case String:
		return bytes.NewReader([]byte(d)), true
	case Bytes:
		return bytes.NewReader([]byte(d)), true
	}
	attr, err := v.GetAttr("read")
	if err != nil {
		return nil, false
	}
	if _, callable := attr.(*Function); !callable {
		return nil, false
	}
	r, err := NewDuckReader(fn, param, v)
	if err != nil {
		return nil, false
	}
	return readOnly{r}, true
}

// readOnly hides the Close of a DuckReader: a library function never closes
// a stream it was handed.
type readOnly struct{ r io.Reader }

func (r readOnly) Read(p []byte) (int, error) { return r.r.Read(p) }

// Data returns a required `data` argument as a reader: a String contributes
// its UTF-8 bytes, Bytes its bytes, and a readable (an object with a
// read(size) method) is read to the end of its stream and never closed. Any
// other value is a TypeError.
func (p *ArgParser) Data(name string) io.Reader {
	v, ok := p.required(name)
	if !ok {
		return nil
	}
	r, ok := DataReader(p.funcName, name, v)
	if !ok {
		p.typeErr(name, "String, Bytes or have a read(size) method", v)
		return nil
	}
	return r
}

// DataBytes is Data read to the end, for a function that needs the whole
// input at once. A String or Bytes is used as it is; a stream is read here,
// and a failure to read it is what Finish reports.
func (p *ArgParser) DataBytes(name string) []byte {
	v, ok := p.required(name)
	if !ok {
		return nil
	}
	switch d := v.(type) {
	case String:
		return []byte(d)
	case Bytes:
		return []byte(d)
	}
	r, ok := DataReader(p.funcName, name, v)
	if !ok {
		p.typeErr(name, "String, Bytes or have a read(size) method", v)
		return nil
	}
	data, err := io.ReadAll(r)
	if err != nil && !p.failed {
		p.failed = true
		p.dataErr = err
		return nil
	}
	return data
}
