package fs

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

type File struct {
	object.OpaqueBase
	Name   string
	File   *os.File
	closed bool
	// reader buffers reads once line iteration has started, and read()
	// continues from it, so a loop that stops early leaves the rest of the
	// file, and nothing else, to be read.
	reader *bufio.Reader
}

func NewFile(name string, file *os.File) *File {
	return &File{OpaqueBase: object.MakeOpaqueBase("File"), Name: name, File: file}
}

// source is where reads come from: the line buffer while one is active,
// otherwise the file itself.
func (f *File) source() io.Reader {
	if f.reader != nil {
		return f.reader
	}
	return f.File
}

// dropReader discards the line buffer before the file position matters to
// something other than a read. The buffer has read ahead of what was
// consumed, so the file is moved back to the first unconsumed byte.
func (f *File) dropReader() error {
	if f.reader == nil {
		return nil
	}
	n := f.reader.Buffered()
	f.reader = nil
	if n > 0 {
		if _, err := f.File.Seek(-int64(n), io.SeekCurrent); err != nil {
			return err
		}
	}
	return nil
}

func (f *File) ensureOpen(method string) error {
	if f.closed {
		return object.NewValueError("%s() called on closed file", method)
	}
	return nil
}

// Read implements the canonical reader shape (STDLIB_DESIGN §5, matching
// http.Body): read() consumes the whole rest of the file, read(size) returns
// a chunk of up to size bytes, and end of stream is an empty Bytes. It
// returns Bytes; call .decode() when the content is known to be text.
func (f *File) Read(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("read", args)
	size, hasSize := p.OptionalInt("size")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("read"); err != nil {
		return nil, err
	}

	if !hasSize {
		data, err := io.ReadAll(f.source())
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, "read() failed to read file", err)
		}
		return object.NewBytes(data), nil
	}
	if size < 0 {
		return nil, object.NewValueError("read() argument 'size' must be non-negative")
	}
	buf := make([]byte, int(size))
	n, err := f.source().Read(buf)
	if err != nil && err != io.EOF {
		return nil, object.WrapNativeError(object.IOError, "read() failed to read file", err)
	}
	return object.NewBytes(buf[:n]), nil
}

// Iter returns the rest of the file as text lines; see Iterator.
func (f *File) Iter() ([]object.Object, error) { return object.Collect(f) }

// Iterator makes `for line in file` read the rest of the file as text lines,
// one line per element, so a loop over a large file holds one line at a time
// and a loop that stops early reads little more than it used. A line ends at
// "\n" or "\r\n", which is not part of it, and a last line without a
// terminator still counts. A line that is not valid UTF-8 raises ValueError
// when the loop reaches it.
func (f *File) Iterator() (object.Iterator, error) {
	if f.closed {
		return nil, object.NewValueError("cannot iterate a closed file")
	}
	line, done := 0, false
	return object.IteratorFunc(func() (object.Object, bool, error) {
		if done {
			return nil, false, nil
		}
		if f.closed {
			return nil, false, object.NewValueError("cannot iterate a closed file")
		}
		if f.reader == nil {
			f.reader = bufio.NewReader(f.File)
		}
		text, err := f.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, false, object.WrapNativeError(object.IOError, "failed to read file lines", err)
		}
		if err == io.EOF {
			done = true
			if text == "" {
				return nil, false, nil
			}
		}
		line++
		if !utf8.ValidString(text) {
			return nil, false, object.NewValueError("line %d of %s is not valid UTF-8", line, f.Name)
		}
		text = strings.TrimSuffix(text, "\n")
		text = strings.TrimSuffix(text, "\r")
		return object.String(text), true, nil
	}), nil
}

func (f *File) Write(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("write", args)
	content := ap.BytesLike("content")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("write"); err != nil {
		return nil, err
	}

	if err := f.dropReader(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write() failed to write file", err)
	}
	n, err := f.File.Write(content)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "write() failed to write file", err)
	}
	return object.Integer(n), nil
}

func (f *File) Close(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("close", args).Finish(); err != nil {
		return nil, err
	}
	if f.closed {
		return object.Nil, nil
	}
	f.reader = nil
	if err := f.File.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "close() failed to close file", err)
	}
	f.closed = true
	return object.Nil, nil
}

func (f *File) Stat(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("stat", args).Finish(); err != nil {
		return nil, err
	}
	if err := f.ensureOpen("stat"); err != nil {
		return nil, err
	}

	info, err := f.File.Stat()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "stat() failed to stat file", err)
	}
	return NewFileInfo(info), nil
}

func (f *File) String() string {
	return fmt.Sprintf("<file %s>", f.Name)
}

func (f *File) ToString() (string, error) { return f.String(), nil }

func (f *File) ToBool() (bool, error) { return !f.closed, nil }

func (f *File) Equals(other object.Object) (bool, error) {
	v, ok := other.(*File)
	return ok && f == v, nil
}

func (f *File) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.String(f.Name), nil
	case "closed":
		return object.Bool(f.closed), nil
	case "read":
		return &object.Function{Name: "read", Fn: f.Read}, nil
	case "write":
		return &object.Function{Name: "write", Fn: f.Write}, nil
	case "stat":
		return &object.Function{Name: "stat", Fn: f.Stat}, nil
	case "close":
		return &object.Function{Name: "close", Fn: f.Close}, nil
	default:
		return nil, object.NewAttributeError("File has no attribute '%s'", name)
	}
}

func (f *File) Attributes() []string {
	return []string{"name", "closed", "read", "write", "stat", "close"}
}

var _ object.Object = (*File)(nil)
