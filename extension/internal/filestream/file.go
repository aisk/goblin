// Package filestream holds the File stream type shared by the modules that
// hand out open files and pipes (fs, os, exec).
package filestream

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

// Type is the type name fs exports for File. A File is never built from
// arguments; it comes from fs.open, the os standard streams or exec pipes.
var Type = object.NewOpaqueConstructor("File", "fs.open(path)")

// File is a readable or writable stream: an open file, a standard stream of
// the process, or a pipe to a child process.
type File struct {
	object.OpaqueBase
	mu sync.Mutex
	// path is the Path the file was opened with, nil for streams and pipes.
	path *pathvalue.Path
	// label names a stream without a path in its Show form.
	label  string
	r      io.Reader
	w      io.Writer
	c      io.Closer
	closed bool
	// lines buffers reads once line reading has started; read() continues
	// from it, so a loop that stops early leaves the rest of the stream, and
	// nothing else, to be read.
	lines *bufio.Reader
	// lineNo counts the lines read so far, for error messages.
	lineNo int
}

var _ object.Object = (*File)(nil)

// Open wraps a file opened at path. It is readable unless writable is set.
func Open(path string, file *os.File, writable bool) *File {
	f := &File{OpaqueBase: object.MakeOpaqueBase("File"), path: pathvalue.NewPath(path), c: file}
	if writable {
		f.w = file
	} else {
		f.r = file
	}
	return f
}

// NewReader wraps a readable stream without a path (stdin, a pipe from a
// child); label is how it shows.
func NewReader(label string, r io.Reader, c io.Closer) *File {
	return &File{OpaqueBase: object.MakeOpaqueBase("File"), label: label, r: r, c: c}
}

// NewWriter wraps a writable stream without a path (stdout, a pipe to a
// child); label is how it shows.
func NewWriter(label string, w io.Writer, c io.Closer) *File {
	return &File{OpaqueBase: object.MakeOpaqueBase("File"), label: label, w: w, c: c}
}

func (f *File) Constructor() *object.Function { return Type.Function }

func (f *File) String() string {
	if f.path != nil {
		return "<File " + f.path.String() + ">"
	}
	return "<File " + f.label + ">"
}

func (f *File) ToString() (string, error) { return f.String(), nil }

func (f *File) Equals(other object.Object) (bool, error) {
	v, ok := other.(*File)
	return ok && f == v, nil
}

// where names the stream in error messages: its path quoted, or its label.
func (f *File) where() string {
	if f.path != nil {
		return `"` + f.path.String() + `"`
	}
	return f.label
}

func (f *File) readable(method string) error {
	if method == "" {
		if f.closed {
			return object.NewValueError("cannot iterate a closed file")
		}
		if f.r == nil {
			return object.NewValueError("cannot iterate a file not open for reading")
		}
		return nil
	}
	if f.closed {
		return object.NewValueError("%s() called on closed file", method)
	}
	if f.r == nil {
		return object.NewValueError("%s() file is not open for reading", method)
	}
	return nil
}

func (f *File) source() io.Reader {
	if f.lines != nil {
		return f.lines
	}
	return f.r
}

func ioError(method, verb string, err error) error {
	var e *object.Error
	if errors.As(err, &e) {
		return err
	}
	return object.WrapNativeError(object.IOError, method+"() failed to "+verb, err)
}

// Read implements the reader shape: read() returns the rest of the stream,
// read(size) up to size bytes, and an empty Bytes at end of stream.
func (f *File) Read(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("read", args)
	size, hasSize := p.NullableInt("size")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if hasSize && size < 0 {
		return nil, object.NewValueError("read() argument 'size' must be non-negative, got %d", int64(size))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.readable("read"); err != nil {
		return nil, err
	}
	if !hasSize {
		data, err := io.ReadAll(f.source())
		if err != nil {
			return nil, ioError("read", "read "+f.where(), err)
		}
		return object.NewBytes(data), nil
	}
	if size == 0 {
		return object.NewBytes(nil), nil
	}
	buf := make([]byte, int(size))
	for {
		n, err := f.source().Read(buf)
		if n > 0 || err == io.EOF {
			return object.NewBytes(buf[:n]), nil
		}
		if err != nil {
			return nil, ioError("read", "read "+f.where(), err)
		}
	}
}

// nextLine returns the next line without its terminator, or false at end
// of stream. A line ends at "\n" or "\r\n"; a last line without one counts.
func (f *File) nextLine(method string) (string, bool, error) {
	if err := f.readable(method); err != nil {
		return "", false, err
	}
	if f.lines == nil {
		// A stream that is already a large enough bufio.Reader (os.stdin's
		// object.Stdin) is used as it is, so its buffer stays shared.
		f.lines = bufio.NewReader(f.r)
	}
	text, ok, err := object.ReadLine(f.lines)
	if err != nil {
		if method == "" {
			return "", false, object.WrapNativeError(object.IOError, "failed to read lines of "+f.where(), err)
		}
		return "", false, ioError(method, "read "+f.where(), err)
	}
	if !ok {
		return "", false, nil
	}
	f.lineNo++
	if !utf8.ValidString(text) {
		return "", false, object.NewValueError("line %d of %s is not valid UTF-8", f.lineNo, f.where())
	}
	return text, true, nil
}

// ReadLine returns the next line, or nil at end of stream.
func (f *File) ReadLine(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("read_line", args); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	line, ok, err := f.nextLine("read_line")
	if err != nil || !ok {
		return object.Nil, err
	}
	return object.String(line), nil
}

// Iter returns the rest of the stream as text lines; see Iterator.
func (f *File) Iter() ([]object.Object, error) { return object.Collect(f) }

// Iterator makes `for line in file` read the rest of the stream one line at
// a time, as read_line does. The loop does not close the file.
func (f *File) Iterator() (object.Iterator, error) {
	f.mu.Lock()
	err := f.readable("")
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return object.IteratorFunc(func() (object.Object, bool, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		line, ok, err := f.nextLine("")
		if err != nil || !ok {
			return nil, false, err
		}
		return object.String(line), true, nil
	}), nil
}

// Write writes a String or Bytes and returns the number of bytes written.
func (f *File) Write(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("write", args)
	data := ap.BytesLike("data")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, object.NewValueError("write() called on closed file")
	}
	if f.w == nil {
		return nil, object.NewValueError("write() file is not open for writing")
	}
	n, err := f.w.Write(data)
	if err != nil {
		return nil, ioError("write", "write "+f.where(), err)
	}
	return object.Integer(n), nil
}

// Close releases the stream; closing it again does nothing.
func (f *File) Close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("close", args); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return object.Nil, nil
	}
	f.closed = true
	f.lines = nil
	if f.c != nil {
		if err := f.c.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			return nil, ioError("close", "close "+f.where(), err)
		}
	}
	return object.Nil, nil
}

// Closed reports whether close() has been called.
func (f *File) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

func (f *File) GetAttr(name string) (object.Object, error) {
	switch name {
	case "path":
		if f.path == nil {
			return object.Nil, nil
		}
		return f.path, nil
	case "closed":
		return object.Bool(f.Closed()), nil
	case "read":
		return &object.Function{Name: "read", Fn: f.Read}, nil
	case "read_line":
		return &object.Function{Name: "read_line", Fn: f.ReadLine}, nil
	case "write":
		return &object.Function{Name: "write", Fn: f.Write}, nil
	case "close":
		return &object.Function{Name: "close", Fn: f.Close}, nil
	}
	return nil, object.NewAttributeError("File has no attribute '%s'", name)
}

func (f *File) Attributes() []string {
	return []string{"path", "closed", "read", "read_line", "write", "close"}
}
