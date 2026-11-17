package fs

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

var (
	linesType = object.NewOpaqueConstructor("Lines", "fs.read_lines(path)")
	walkType  = object.NewOpaqueConstructor("Walk", "fs.walk(path)")
)

// Lines is what fs.read_lines returns: each pass opens the file, yields its
// lines and closes it when the pass ends, however it ends.
type Lines struct {
	object.OpaqueBase
	path string
}

func readLines(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("read_lines", args)
	if err != nil {
		return nil, err
	}
	return &Lines{OpaqueBase: object.MakeOpaqueBase("Lines"), path: path}, nil
}

func (l *Lines) Constructor() *object.Function { return linesType.Function }
func (l *Lines) String() string                { return "<Lines " + l.path + ">" }
func (l *Lines) ToString() (string, error)     { return l.String(), nil }
func (l *Lines) Attributes() []string          { return nil }
func (l *Lines) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Lines has no attribute '%s'", name)
}

func (l *Lines) Iter() ([]object.Object, error) { return object.Collect(l) }

// openLines opens the file of one read_lines pass; tests replace it to see
// the file closed.
var openLines = func(path string) (io.ReadCloser, error) { return os.Open(path) }

func (l *Lines) Iterator() (object.Iterator, error) {
	file, err := openLines(l.path)
	if err != nil {
		return nil, fail("read_lines", "cannot open", l.path, err)
	}
	reader := bufio.NewReader(file)
	line := 0
	release := func() {
		if file != nil {
			file.Close()
			file = nil
		}
	}
	return object.IteratorWithClose(func() (object.Object, bool, error) {
		if file == nil {
			return nil, false, nil
		}
		text, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			release()
			return nil, false, fail("read_lines", "cannot read", l.path, err)
		}
		if err == io.EOF && text == "" {
			release()
			return nil, false, nil
		}
		line++
		if !utf8.ValidString(text) {
			release()
			return nil, false, object.NewValueError("line %d of %s is not valid UTF-8", line, quote(l.path))
		}
		text = strings.TrimSuffix(text, "\n")
		text = strings.TrimSuffix(text, "\r")
		return object.String(text), true, nil
	}, release), nil
}

// Walk is what fs.walk returns: each pass yields every file below root,
// depth first with names sorted within each directory.
type Walk struct {
	object.OpaqueBase
	root string
	skip *object.Function
}

func walk(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("walk", args)
	root := p.Path("path")
	skip, _ := p.NullableFunc("skip")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return &Walk{OpaqueBase: object.MakeOpaqueBase("Walk"), root: root, skip: skip}, nil
}

func (w *Walk) Constructor() *object.Function { return walkType.Function }
func (w *Walk) String() string                { return "<Walk " + w.root + ">" }
func (w *Walk) ToString() (string, error)     { return w.String(), nil }
func (w *Walk) Attributes() []string          { return nil }
func (w *Walk) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Walk has no attribute '%s'", name)
}

func (w *Walk) Iter() ([]object.Object, error) { return object.Collect(w) }

// walkFrame is one directory being listed: its path and the entries not yet
// visited.
type walkFrame struct {
	dir     string
	entries []os.DirEntry
}

func (w *Walk) Iterator() (object.Iterator, error) {
	entries, err := dirEntries("walk", w.root)
	if err != nil {
		return nil, err
	}
	stack := []walkFrame{{dir: w.root, entries: entries}}
	return object.IteratorWithClose(func() (object.Object, bool, error) {
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if len(top.entries) == 0 {
				stack = stack[:len(stack)-1]
				continue
			}
			entry := top.entries[0]
			top.entries = top.entries[1:]
			path := pathvalue.NewPath(filepath.Join(top.dir, entry.Name()))
			if w.skip != nil {
				verdict, err := w.skip.Call(object.CallArgs{Positional: object.Args{path}})
				if err != nil {
					stack = nil
					return nil, false, err
				}
				skipped, err := verdict.ToBool()
				if err != nil {
					stack = nil
					return nil, false, err
				}
				if skipped {
					continue
				}
			}
			// DirEntry.IsDir does not follow symlinks, so a link is
			// yielded and never descended.
			if entry.IsDir() {
				children, err := os.ReadDir(path.FilePath())
				if err != nil {
					stack = nil
					return nil, false, fail("walk", "cannot read", path.FilePath(), err)
				}
				stack = append(stack, walkFrame{dir: path.FilePath(), entries: children})
				continue
			}
			return path, true, nil
		}
		return nil, false, nil
	}, func() { stack = nil }), nil
}
