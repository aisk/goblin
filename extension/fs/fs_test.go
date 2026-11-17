package fs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

func member(t *testing.T, name string) object.Object {
	t.Helper()
	module, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := module.(*object.Module).Members[name]
	if !ok {
		t.Fatalf("fs has no member %q", name)
	}
	return m
}

func args(positional []object.Object, kw ...object.KwArg) object.CallArgs {
	return object.CallArgs{Positional: positional, Keyword: kw}
}

func call(t *testing.T, name string, positional ...object.Object) object.Object {
	t.Helper()
	return modtest.CallModule(t, Execute, name, args(positional))
}

func callKw(t *testing.T, name string, positional []object.Object, kw ...object.KwArg) object.Object {
	t.Helper()
	return modtest.CallModule(t, Execute, name, args(positional, kw...))
}

// callErr calls a member expecting an error of kind and a message containing
// want.
func callErr(t *testing.T, kind *object.Error, want string, name string, positional []object.Object, kw ...object.KwArg) {
	t.Helper()
	_, err := member(t, name).(*object.Function).Call(args(positional, kw...))
	if err == nil {
		t.Fatalf("%s() succeeded, want %s", name, kind)
	}
	if !errors.Is(err, kind) {
		t.Fatalf("%s() error = %v, want kind %s", name, err, kind)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("%s() error = %q, want it to contain %q", name, err.Error(), want)
	}
}

func str(s string) object.Object { return object.String(s) }

func kw(name string, v object.Object) object.KwArg { return object.KwArg{Name: name, Value: v} }

func method(t *testing.T, v object.Object, name string, positional ...object.Object) object.Object {
	t.Helper()
	got, err := methodErr(v, name, positional...)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return got
}

func methodErr(v object.Object, name string, positional ...object.Object) (object.Object, error) {
	attr, err := v.GetAttr(name)
	if err != nil {
		return nil, err
	}
	return attr.(*object.Function).Call(object.CallArgs{Positional: positional})
}

func attr(t *testing.T, v object.Object, name string) object.Object {
	t.Helper()
	got, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func collect(t *testing.T, v object.Object) []string {
	t.Helper()
	items, err := object.Collect(v)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(items))
	for i, item := range items {
		s, _ := item.ToString()
		out[i] = filepath.ToSlash(s)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if got := callKw(t, "write", []object.Object{str(path), str("one\n")}); got != object.Nil {
		t.Fatalf("write() = %v, want nil", got)
	}
	callKw(t, "write", []object.Object{str(path), object.Bytes("two\n")}, kw("append", object.True))
	if got := call(t, "read", pathvalue.NewPath(path)); got != object.String("one\ntwo\n") {
		t.Fatalf("read() = %q", got)
	}
	if got := call(t, "read_bytes", str(path)); !modtest.Equal(got, object.Bytes("one\ntwo\n")) {
		t.Fatalf("read_bytes() = %v", got)
	}

	empty := filepath.Join(dir, "empty")
	call(t, "write", str(empty), str(""))
	if got := call(t, "read", str(empty)); got != object.String("") {
		t.Fatalf("read(empty) = %q", got)
	}

	var chunks bytes.Buffer
	chunks.WriteString("streamed")
	reader := &object.Module{Name: "reader", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(a object.CallArgs) (object.Object, error) {
			buf := make([]byte, 3)
			n, _ := chunks.Read(buf)
			return object.Bytes(buf[:n]), nil
		}},
	}}
	call(t, "write", str(path), reader)
	if got := call(t, "read", str(path)); got != object.String("streamed") {
		t.Fatalf("read() after streamed write = %q", got)
	}

	bad := filepath.Join(dir, "bad")
	writeFile(t, bad, "ok\xff")
	callErr(t, object.ValueError, `read() "`+bad+`" is not valid UTF-8`, "read", []object.Object{str(bad)})
	callErr(t, object.NotExistError, `read() cannot open "`+filepath.Join(dir, "nope")+`"`, "read", []object.Object{str(filepath.Join(dir, "nope"))})
	callErr(t, object.NotExistError, "write() cannot open", "write", []object.Object{str(filepath.Join(dir, "no", "x")), str("")})
	callErr(t, object.TypeError, "read() argument 'path' must be String or Path, got Int", "read", []object.Object{object.Integer(1)})
	callErr(t, object.TypeError, "write() argument 'data' must be String, Bytes or have a read(size) method, got Int", "write", []object.Object{str(path), object.Integer(1)})
}

type trackedFile struct {
	io.ReadCloser
	closed *int
}

func (f trackedFile) Close() error {
	*f.closed++
	return f.ReadCloser.Close()
}

func TestReadLinesClosesOnEveryExit(t *testing.T) {
	closed := 0
	saved := openLines
	openLines = func(path string) (io.ReadCloser, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return trackedFile{f, &closed}, nil
	}
	defer func() { openLines = saved }()

	path := filepath.Join(t.TempDir(), "lines.txt")
	writeFile(t, path, "a\r\nb\n\nc")
	lines := call(t, "read_lines", str(path))
	if got := collect(t, lines); !reflect.DeepEqual(got, []string{"a", "b", "", "c"}) {
		t.Fatalf("lines = %q", got)
	}
	if closed != 1 {
		t.Fatalf("file closed %d times after a full pass, want 1", closed)
	}

	// A second pass opens the file again; stopping early closes it.
	it, err := object.Iterate(lines)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok, err := it.Next(); err != nil || !ok || v != object.String("a") {
		t.Fatalf("first line = %v, %v, %v", v, ok, err)
	}
	it.Close()
	if closed != 2 {
		t.Fatalf("file closed %d times after break, want 2", closed)
	}

	bad := filepath.Join(t.TempDir(), "bad.txt")
	writeFile(t, bad, "ok\nbad\xff\n")
	_, err = object.Collect(call(t, "read_lines", str(bad)))
	if err == nil || !errors.Is(err, object.ValueError) || !strings.Contains(err.Error(), `line 2 of "`+bad+`" is not valid UTF-8`) {
		t.Fatalf("invalid line error = %v", err)
	}
	if closed != 3 {
		t.Fatalf("file closed %d times after an error, want 3", closed)
	}

	_, err = object.Iterate(call(t, "read_lines", str(filepath.Join(t.TempDir(), "missing"))))
	if !errors.Is(err, object.NotExistError) {
		t.Fatalf("read_lines(missing) pass error = %v", err)
	}
}

func makeTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	writeFile(t, filepath.Join(dir, "a", "z.md"), "z")
	writeFile(t, filepath.Join(dir, "a", "deep", "y.md"), "y")
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref")
	return dir
}

func relative(t *testing.T, dir string, paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		rel, err := filepath.Rel(dir, filepath.FromSlash(p))
		if err != nil {
			t.Fatal(err)
		}
		out[i] = filepath.ToSlash(rel)
	}
	return out
}

func TestWalk(t *testing.T) {
	dir := makeTree(t)
	got := relative(t, dir, collect(t, call(t, "walk", str(dir))))
	want := []string{".git/HEAD", "a/deep/y.md", "a/z.md", "b.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walk() = %q, want %q", got, want)
	}

	var asked []string
	skip := &object.Function{Name: "skip", Fn: func(a object.CallArgs) (object.Object, error) {
		name := a.Positional[0].(*pathvalue.Path)
		n, _ := name.GetAttr("name")
		asked = append(asked, string(n.(object.String)))
		return object.Bool(n == object.String(".git") || n == object.String("deep")), nil
	}}
	got = relative(t, dir, collect(t, callKw(t, "walk", []object.Object{str(dir)}, kw("skip", skip))))
	if want := []string{"a/z.md", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("walk(skip=) = %q, want %q", got, want)
	}
	if want := []string{".git", "a", "deep", "z.md", "b.txt"}; !reflect.DeepEqual(asked, want) {
		t.Fatalf("skip was asked about %q, want %q", asked, want)
	}

	// An early stop is fine and a new pass starts over.
	it, err := object.Iterate(call(t, "walk", str(dir)))
	if err != nil {
		t.Fatal(err)
	}
	it.Next()
	it.Close()

	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(dir, "a"), filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		got = relative(t, dir, collect(t, call(t, "walk", str(dir))))
		if want := []string{".git/HEAD", "a/deep/y.md", "a/z.md", "b.txt", "link"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("walk() with a symlink = %q, want %q", got, want)
		}
	}

	_, err = object.Iterate(call(t, "walk", str(filepath.Join(dir, "missing"))))
	if !errors.Is(err, object.NotExistError) {
		t.Fatalf("walk(missing) error = %v", err)
	}
}

func TestGlob(t *testing.T) {
	dir := makeTree(t)
	glob := func(pattern string) []string {
		return relative(t, dir, collect(t, call(t, "glob", str(filepath.Join(dir, pattern)))))
	}
	if got, want := glob("**/*.md"), []string{"a/deep/y.md", "a/z.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("glob(**/*.md) = %q, want %q", got, want)
	}
	if got, want := glob("a/**/*.md"), []string{"a/deep/y.md", "a/z.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("glob(a/**/*.md) = %q, want %q", got, want)
	}
	if got, want := glob("?.txt"), []string{"b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("glob(?.txt) = %q, want %q", got, want)
	}
	if got, want := glob("[a-b]*"), []string{"a", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("glob([a-b]*) = %q, want %q", got, want)
	}
	if got := glob("*.none"); len(got) != 0 {
		t.Fatalf("glob(no match) = %q", got)
	}
	callErr(t, object.ValueError, `glob() malformed pattern "[a"`, "glob", []object.Object{str("[a")})

	// A relative pattern is relative to the working directory.
	t.Chdir(dir)
	if got, want := collect(t, call(t, "glob", str("a/*.md"))), []string{"a/z.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("relative glob = %q, want %q", got, want)
	}
}

func TestQuestionsAndStat(t *testing.T) {
	dir := makeTree(t)
	file := filepath.Join(dir, "b.txt")
	for _, c := range []struct {
		fn, path string
		want     bool
	}{
		{"exists", file, true}, {"exists", dir, true}, {"exists", filepath.Join(dir, "x"), false},
		{"exists", filepath.Join(file, "x"), false},
		{"is_file", file, true}, {"is_file", dir, false}, {"is_file", filepath.Join(dir, "x"), false},
		{"is_dir", dir, true}, {"is_dir", file, false}, {"is_dir", filepath.Join(dir, "x"), false},
	} {
		if got := call(t, c.fn, str(c.path)); got != object.Bool(c.want) {
			t.Errorf("%s(%s) = %v, want %v", c.fn, c.path, got, c.want)
		}
	}

	info := call(t, "stat", str(file))
	if attr(t, info, "size") != object.Integer(1) || attr(t, info, "is_file") != object.True ||
		attr(t, info, "is_dir") != object.False || attr(t, info, "is_symlink") != object.False {
		t.Fatalf("stat(file) = %v", info)
	}
	if runtime.GOOS != "windows" {
		if attr(t, info, "mode") != object.Integer(0o644) {
			t.Fatalf("mode = %v", attr(t, info, "mode"))
		}
	}
	if _, ok := attr(t, info, "modified_at").(object.Constructed); !ok {
		t.Fatalf("modified_at is %T", attr(t, info, "modified_at"))
	}
	if got := object.TypeOf(info); got != member(t, "FileInfo") {
		t.Fatal("type_of(stat()) is not fs.FileInfo")
	}
	callErr(t, object.NotExistError, "stat() cannot stat", "stat", []object.Object{str(filepath.Join(dir, "x"))})

	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		os.Symlink(file, link)
		info = call(t, "stat", str(link))
		if attr(t, info, "is_symlink") != object.True || attr(t, info, "is_file") != object.True {
			t.Fatal("stat(link) does not describe a link to a file")
		}
		broken := filepath.Join(dir, "broken")
		os.Symlink(filepath.Join(dir, "gone"), broken)
		info = call(t, "stat", str(broken))
		if attr(t, info, "is_symlink") != object.True || attr(t, info, "is_file") != object.False {
			t.Fatal("stat(broken link) does not describe the link")
		}
		if call(t, "exists", str(broken)) != object.False {
			t.Fatal("exists(broken link) is true")
		}
	}
}

func TestReadDirMkdirRemove(t *testing.T) {
	dir := makeTree(t)
	got := relative(t, dir, collect(t, call(t, "read_dir", str(dir))))
	if want := []string{".git", "a", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("read_dir() = %q, want %q", got, want)
	}
	empty := filepath.Join(dir, "x", "y")
	call(t, "mkdir", str(empty))
	call(t, "mkdir", str(empty))
	if got := collect(t, call(t, "read_dir", str(empty))); len(got) != 0 {
		t.Fatalf("read_dir(empty) = %q", got)
	}
	file := filepath.Join(dir, "b.txt")
	callErr(t, object.IOError, `read_dir() "`+file+`" is not a directory`, "read_dir", []object.Object{str(file)})
	callErr(t, object.ExistError, "mkdir() cannot create", "mkdir", []object.Object{str(file)})
	callErr(t, object.ExistError, "mkdir() cannot create", "mkdir", []object.Object{str(filepath.Join(file, "sub"))})

	call(t, "remove", str(filepath.Join(dir, "missing")))
	call(t, "remove", str(empty))
	callErr(t, object.IOError, `remove() directory "`+filepath.Join(dir, "a")+`" is not empty (pass recursive=true)`, "remove", []object.Object{str(filepath.Join(dir, "a"))})
	callKw(t, "remove", []object.Object{str(filepath.Join(dir, "a"))}, kw("recursive", object.True))
	call(t, "remove", str(file))
	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Fatal("recursive remove left the directory")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("remove left the file")
	}

	temp := callKw(t, "mkdir_temp", nil, kw("prefix", str("goblin-test-")))
	defer os.RemoveAll(temp.(*pathvalue.Path).FilePath())
	name := attr(t, temp, "name").(object.String)
	if !strings.HasPrefix(string(name), "goblin-test-") || call(t, "is_dir", temp) != object.True {
		t.Fatalf("mkdir_temp() = %v", temp)
	}
	if attr(t, temp, "parent").(*pathvalue.Path).FilePath() != filepath.Clean(os.TempDir()) {
		t.Fatalf("mkdir_temp() is not under temp_dir(): %v", temp)
	}
}

func TestCopyMoveChmod(t *testing.T) {
	dir := makeTree(t)
	file := filepath.Join(dir, "b.txt")
	if runtime.GOOS != "windows" {
		call(t, "chmod", str(file), object.Integer(0o600))
	}
	copied := filepath.Join(dir, "c.txt")
	call(t, "copy", str(file), str(copied))
	call(t, "copy", str(file), str(copied))
	if data, _ := os.ReadFile(copied); string(data) != "b" {
		t.Fatalf("copy wrote %q", data)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(copied); info.Mode().Perm() != 0o600 {
			t.Fatalf("copy did not keep the mode: %v", info.Mode())
		}
	}
	callErr(t, object.ExistError, "is a directory", "copy", []object.Object{str(file), str(filepath.Join(dir, "a"))})
	callErr(t, object.IOError, "are the same file", "copy", []object.Object{str(file), str(file)})
	callErr(t, object.NotExistError, "copy() cannot copy", "copy", []object.Object{str(filepath.Join(dir, "x")), str(copied)})
	callErr(t, object.NotExistError, "copy() cannot copy", "copy", []object.Object{str(file), str(filepath.Join(dir, "no", "x"))})

	call(t, "copy", str(filepath.Join(dir, "a")), str(filepath.Join(dir, "a2")))
	if data, _ := os.ReadFile(filepath.Join(dir, "a2", "deep", "y.md")); string(data) != "y" {
		t.Fatal("directory copy is missing a nested file")
	}
	callErr(t, object.ExistError, "already exists", "copy", []object.Object{str(filepath.Join(dir, "a")), str(filepath.Join(dir, "a2"))})
	// Copying a directory into its own tree, at any depth, is refused
	// before anything is written.
	for _, into := range []string{
		filepath.Join(dir, "a", "inner"),
		filepath.Join(dir, "a", "deep", "out"),
		filepath.Join(dir, "a", "deep", "..", "deep", "out"),
	} {
		callErr(t, object.IOError, "into itself", "copy", []object.Object{str(filepath.Join(dir, "a")), str(into)})
		if _, err := os.Stat(into); !os.IsNotExist(err) {
			t.Fatalf("refused copy into itself still created %s", into)
		}
	}
	if path := errorPath(t, "copy", str(filepath.Join(dir, "a")), str(filepath.Join(dir, "a", "inner"))); path != filepath.Join(dir, "a", "inner") {
		t.Fatalf("copy into itself carries path %q", path)
	}
	// A sibling whose name starts with the source's is not inside it.
	call(t, "copy", str(filepath.Join(dir, "a")), str(filepath.Join(dir, "ab")))

	moved := filepath.Join(dir, "a2", "moved.txt")
	call(t, "move", str(copied), str(moved))
	if _, err := os.Stat(copied); !os.IsNotExist(err) {
		t.Fatal("move left the source")
	}
	writeFile(t, copied, "new")
	call(t, "move", str(copied), str(moved))
	if data, _ := os.ReadFile(moved); string(data) != "new" {
		t.Fatal("move did not replace an existing file")
	}
	callErr(t, object.ExistError, "is a directory", "move", []object.Object{str(moved), str(filepath.Join(dir, "a"))})
	callErr(t, object.NotExistError, "move() cannot move", "move", []object.Object{str(copied), str(moved)})

	callErr(t, object.ValueError, "between 0 and 0o777", "chmod", []object.Object{str(file), object.Integer(0o1000)})
	callErr(t, object.NotExistError, "chmod()", "chmod", []object.Object{str(copied), object.Integer(0o644)})
}

func TestLocations(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	resolved := call(t, "resolve", str("missing/../x")).(*pathvalue.Path)
	cwd := call(t, "cwd").(*pathvalue.Path)
	realCwd, _ := filepath.EvalSymlinks(cwd.FilePath())
	if resolved.FilePath() != filepath.Join(realCwd, "x") {
		t.Fatalf("resolve() = %s, want %s", resolved, filepath.Join(realCwd, "x"))
	}
	if !filepath.IsAbs(call(t, "home").(*pathvalue.Path).FilePath()) {
		t.Fatal("home() is not absolute")
	}
	if call(t, "temp_dir").(*pathvalue.Path).FilePath() != filepath.Clean(os.TempDir()) {
		t.Fatal("temp_dir() is not os.TempDir()")
	}
}

func TestFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	out := callKw(t, "open", []object.Object{str(path)}, kw("write", object.True))
	if got := method(t, out, "write", str("héllo\nworld\n")); got != object.Integer(13) {
		t.Fatalf("write() = %v", got)
	}
	if _, err := methodErr(out, "read"); !errors.Is(err, object.ValueError) {
		t.Fatalf("read() on a write-only file error = %v", err)
	}
	if s, _ := out.ToString(); s != "<File "+path+">" {
		t.Fatalf("Show = %q", s)
	}
	method(t, out, "close")
	method(t, out, "close")
	if attr(t, out, "closed") != object.True {
		t.Fatal("closed is false after close()")
	}
	if _, err := methodErr(out, "write", str("x")); !errors.Is(err, object.ValueError) {
		t.Fatalf("write() after close error = %v", err)
	}
	app := callKw(t, "open", []object.Object{str(path)}, kw("append", object.True))
	method(t, app, "write", object.Bytes("tail"))
	method(t, app, "close")

	in := call(t, "open", str(path))
	if !modtest.Equal(attr(t, in, "path"), pathvalue.NewPath(path)) {
		t.Fatal("path is not the opened Path")
	}
	if object.TypeOf(in) != member(t, "File") {
		t.Fatal("type_of(file) is not fs.File")
	}
	if got := method(t, in, "read", object.Integer(2)); !modtest.Equal(got, object.Bytes("h\xc3")) {
		t.Fatalf("read(2) = %v", got)
	}
	if _, err := methodErr(in, "read", object.Integer(-1)); !errors.Is(err, object.ValueError) {
		t.Fatalf("read(-1) error = %v", err)
	}
	if _, err := methodErr(in, "write", str("x")); !errors.Is(err, object.ValueError) {
		t.Fatalf("write() on a read-only file error = %v", err)
	}
	if _, err := methodErr(in, "read_line"); !errors.Is(err, object.ValueError) {
		t.Fatalf("read_line() through a split character error = %v", err)
	}
	if got := method(t, in, "read_line"); got != object.String("world") {
		t.Fatalf("read_line() = %v", got)
	}
	// A loop takes the remaining lines and leaves the file open.
	if got := collect(t, in); !reflect.DeepEqual(got, []string{"tail"}) {
		t.Fatalf("lines = %q", got)
	}
	if attr(t, in, "closed") != object.False {
		t.Fatal("a loop closed the file")
	}
	if got := method(t, in, "read_line"); got != object.Nil {
		t.Fatalf("read_line() at end = %v", got)
	}
	if got := method(t, in, "read"); !modtest.Equal(got, object.Bytes("")) {
		t.Fatalf("read() at end = %v", got)
	}
	method(t, in, "close")

	callErr(t, object.NotExistError, "open() cannot open", "open", []object.Object{str(filepath.Join(dir, "x"))})
	callErr(t, object.TypeError, "File cannot be constructed; use fs.open(path)", "File", nil)
	callErr(t, object.TypeError, "FileInfo cannot be constructed", "FileInfo", nil)
}

func TestPath(t *testing.T) {
	p := func(s string) *pathvalue.Path {
		return call(t, "Path", str(filepath.FromSlash(s))).(*pathvalue.Path)
	}
	text := func(v object.Object) string {
		s, _ := v.ToString()
		return filepath.ToSlash(s)
	}
	for _, c := range []struct{ path, name, stem, suffix, parent string }{
		{"docs/report.tar.gz", "report.tar.gz", "report.tar", ".gz", "docs"},
		{"a//b/./c/../d.md", "d.md", "d", ".md", "a/b"},
		{".bashrc", ".bashrc", ".bashrc", "", "."},
		{"a", "a", "a", "", "."},
		{"", "", "", "", "."},
		{"/", "", "", "", "/"},
		{"..", "..", "..", "", "."},
	} {
		v := p(c.path)
		got := []string{text(attr(t, v, "name")), text(attr(t, v, "stem")), text(attr(t, v, "suffix")), text(attr(t, v, "parent"))}
		if want := []string{c.name, c.stem, c.suffix, c.parent}; !reflect.DeepEqual(got, want) {
			t.Errorf("Path(%q) name/stem/suffix/parent = %q, want %q", c.path, got, want)
		}
	}
	if text(method(t, p("a"), "join", str("/b"), p("c"))) != "a/b/c" {
		t.Fatal("join")
	}
	if method(t, p("a"), "join") == nil {
		t.Fatal("join() with no segments")
	}
	if _, err := methodErr(p("a"), "join", object.Integer(1)); !errors.Is(err, object.TypeError) {
		t.Fatalf("join(1) error = %v", err)
	}
	if text(method(t, p("a/b.txt"), "with_name", str("c.md"))) != "a/c.md" ||
		text(method(t, p("a/b.tar.gz"), "with_suffix", str(".zip"))) != "a/b.tar.zip" ||
		text(method(t, p("a/b.txt"), "with_suffix", str(""))) != "a/b" {
		t.Fatal("with_name / with_suffix")
	}
	for _, bad := range []struct {
		path, method, arg string
	}{
		{"a", "with_name", ""}, {"a", "with_name", "x/y"}, {"/", "with_name", "x"},
		{"a", "with_suffix", "md"}, {"/", "with_suffix", ".md"},
	} {
		if _, err := methodErr(p(bad.path), bad.method, str(bad.arg)); !errors.Is(err, object.ValueError) {
			t.Errorf("Path(%q).%s(%q) error = %v, want ValueError", bad.path, bad.method, bad.arg, err)
		}
	}
	if text(method(t, p("/a/b/c"), "relative_to", str("/a/x"))) != "../b/c" {
		t.Fatal("relative_to")
	}
	if _, err := methodErr(p("/a"), "relative_to", str("b")); !errors.Is(err, object.ValueError) {
		t.Fatalf("relative_to(mixed) error = %v", err)
	}
	if attr(t, p("/a"), "is_absolute") != object.True || attr(t, p("a"), "is_absolute") != object.False {
		t.Fatal("is_absolute")
	}

	if !modtest.Equal(p("a/"), p("a")) || modtest.Equal(p("a"), str("a")) {
		t.Fatal("Path equality")
	}
	h1, _ := object.Hash(p("a/"))
	h2, _ := object.Hash(p("a"))
	if h1 != h2 {
		t.Fatal("equal Paths hash differently")
	}
	if c, err := object.Compare(p("a"), p("b")); err != nil || c >= 0 {
		t.Fatalf("Path order = %d, %v", c, err)
	}
	if truthy, _ := p(".").ToBool(); !truthy {
		t.Fatal("a Path is falsy")
	}
	if _, err := p("a").Divide(str("b")); err == nil {
		t.Fatal("Path supports /")
	}
	if object.TypeOf(p("a")) != member(t, "Path") {
		t.Fatal("type_of(Path) is not fs.Path")
	}
	callErr(t, object.TypeError, "Path() takes 1 positional arguments, got 2", "Path", []object.Object{str("a"), str("b")})
}

// errorPath calls a member expecting an error and returns the text of its
// path data.
func errorPath(t *testing.T, name string, positional ...object.Object) string {
	t.Helper()
	_, err := member(t, name).(*object.Function).Call(args(positional))
	if err == nil {
		t.Fatalf("%s() succeeded", name)
	}
	var goblinErr *object.Error
	if !errors.As(err, &goblinErr) {
		t.Fatalf("%s() error %v is not a Goblin error", name, err)
	}
	p, ok := goblinErr.Field("path")
	if !ok {
		t.Fatalf("%s() error %q carries no path", name, err)
	}
	if _, isPath := p.(*pathvalue.Path); !isPath {
		t.Fatalf("%s() error path = %#v, want a Path", name, p)
	}
	if !strings.Contains(err.Error(), `"`+p.(*pathvalue.Path).FilePath()+`"`) {
		t.Fatalf("%s() error %q does not name its path %v", name, err, p)
	}
	return p.(*pathvalue.Path).FilePath()
}

func TestErrorsCarryPath(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")
	file := filepath.Join(dir, "file.txt")
	writeFile(t, file, "x")
	full := filepath.Join(dir, "full")
	writeFile(t, filepath.Join(full, "a"), "a")
	bad := filepath.Join(dir, "bad.txt")
	writeFile(t, bad, "\xff")

	cases := []struct {
		name string
		args []object.Object
		want string
	}{
		{"read", []object.Object{str(missing)}, missing},
		{"read", []object.Object{str(bad)}, bad},
		{"write", []object.Object{str(filepath.Join(missing, "x")), str("data")}, filepath.Join(missing, "x")},
		{"stat", []object.Object{str(missing)}, missing},
		{"read_dir", []object.Object{str(file)}, file},
		{"copy", []object.Object{str(missing), str(file)}, missing},
		{"copy", []object.Object{str(file), str(filepath.Join(missing, "dst"))}, filepath.Join(missing, "dst")},
		{"copy", []object.Object{str(file), str(file)}, file},
		{"move", []object.Object{str(missing), str(file)}, missing},
		{"remove", []object.Object{str(full)}, full},
		{"mkdir", []object.Object{str(filepath.Join(file, "sub"))}, filepath.Join(file, "sub")},
	}
	for _, c := range cases {
		if got := errorPath(t, c.name, c.args...); got != c.want {
			t.Errorf("%s(%v) path = %q, want %q", c.name, c.args, got, c.want)
		}
	}

	// A line that is not valid UTF-8 carries the line number too, from
	// read_lines and from a File opened from a path.
	lines := filepath.Join(dir, "lines.txt")
	writeFile(t, lines, "one\ntwo\nbad\xff\n")
	_, err := object.Collect(call(t, "read_lines", str(lines)))
	checkLineError(t, err, lines, 3)
	f := call(t, "open", str(lines))
	defer method(t, f, "close")
	_, err = object.Collect(f)
	checkLineError(t, err, lines, 3)
}

func checkLineError(t *testing.T, err error, path string, line int) {
	t.Helper()
	var goblinErr *object.Error
	if !errors.As(err, &goblinErr) || !errors.Is(err, object.ValueError) {
		t.Fatalf("line error = %v, want a ValueError", err)
	}
	if got, _ := goblinErr.Field("line"); got != object.Integer(line) {
		t.Fatalf("line = %v, want %d", got, line)
	}
	if got, _ := goblinErr.Field("path"); got == nil || got.(*pathvalue.Path).FilePath() != path {
		t.Fatalf("path = %v, want %q", got, path)
	}
}
