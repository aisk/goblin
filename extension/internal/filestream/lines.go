package filestream

import (
	"bufio"
	"errors"
	"io"

	"github.com/aisk/goblin/object"
)

// LinesType is the type name fs exports for Lines, the value of
// fs.read_lines and gzip.read_lines.
var LinesType = object.NewOpaqueConstructor("Lines", "fs.read_lines(path)")

// Opener opens the stream one pass of a Lines reads: the reader of the
// text and the closer that releases what was opened. An error it returns
// that is already a Goblin error is raised as it is; any other is reported
// as `fn() cannot open "path"`.
type Opener func() (io.Reader, io.Closer, error)

// Lines is a lazy iterable of the lines of the file at a path: each pass
// opens it, yields its lines and closes it when the pass ends, however it
// ends.
type Lines struct {
	object.OpaqueBase
	fn   string
	path string
	open Opener
}

// NewLines makes the value fn (read_lines) returns for path.
func NewLines(fn, path string, open Opener) *Lines {
	return &Lines{OpaqueBase: object.MakeOpaqueBase("Lines"), fn: fn, path: path, open: open}
}

func (l *Lines) Constructor() *object.Function { return LinesType.Function }
func (l *Lines) String() string                { return "<Lines " + l.path + ">" }
func (l *Lines) ToString() (string, error)     { return l.String(), nil }
func (l *Lines) Attributes() []string          { return nil }
func (l *Lines) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Lines has no attribute '%s'", name)
}

func (l *Lines) Iter() ([]object.Object, error) { return object.Collect(l) }

func (l *Lines) Iterator() (object.Iterator, error) {
	r, c, err := l.open()
	if err != nil {
		// An opener that already reported the failure (gzip's, which names
		// the call and the path) is not wrapped in a second report.
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return nil, err
		}
		return nil, Fail(l.fn, "cannot open", l.path, err)
	}
	reader := bufio.NewReader(r)
	line := 0
	done := false
	release := func() {
		if !done {
			done = true
			c.Close()
		}
	}
	return object.IteratorWithClose(func() (object.Object, bool, error) {
		if done {
			return nil, false, nil
		}
		text, ok, err := object.ReadLine(reader)
		if err != nil {
			release()
			if goblinErr, ok := methodError(l.fn, err); ok {
				return nil, false, goblinErr
			}
			return nil, false, Fail(l.fn, "cannot read", l.path, err)
		}
		if !ok {
			release()
			return nil, false, nil
		}
		line++
		if !validUTF8(text) {
			release()
			return nil, false, InvalidLine(line, Quote(l.path), l.path)
		}
		return object.String(text), true, nil
	}, release), nil
}
