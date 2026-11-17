package fs

import (
	"io"
	"os"
	"path/filepath"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

var walkType = object.NewOpaqueConstructor("Walk", "fs.walk(path)")

// readLines returns the lazy Lines of a file: each pass opens it, yields
// its lines and closes it when the pass ends, however it ends.
func readLines(args object.CallArgs) (object.Object, error) {
	path, err := pathArg("read_lines", args)
	if err != nil {
		return nil, err
	}
	return filestream.NewLines("read_lines", path, func() (io.Reader, io.Closer, error) {
		file, err := openLines(path)
		if err != nil {
			return nil, nil, err
		}
		return file, file, nil
	}), nil
}

// openLines opens the file of one read_lines pass; tests replace it to see
// the file closed.
var openLines = func(path string) (io.ReadCloser, error) { return os.Open(path) }

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
