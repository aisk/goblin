package zip

import (
	gozip "archive/zip"
	"bytes"
	"errors"
	"hash/crc32"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aisk/goblin/extension/internal/archive"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

func module(t *testing.T) *object.Module {
	t.Helper()
	m, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	return m.(*object.Module)
}

func call(t *testing.T, name string, pos []object.Object, kws ...object.KwArg) (object.Object, error) {
	t.Helper()
	return module(t).Members[name].(*object.Function).Call(object.CallArgs{Positional: pos, Keyword: kws})
}

func mustCall(t *testing.T, name string, pos []object.Object, kws ...object.KwArg) object.Object {
	t.Helper()
	v, err := call(t, name, pos, kws...)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return v
}

func invoke(t *testing.T, v object.Object, name string, pos []object.Object, kws ...object.KwArg) (object.Object, error) {
	t.Helper()
	m, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return m.(*object.Function).Call(object.CallArgs{Positional: pos, Keyword: kws})
}

func mustInvoke(t *testing.T, v object.Object, name string, pos []object.Object, kws ...object.KwArg) object.Object {
	t.Helper()
	got, err := invoke(t, v, name, pos, kws...)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return got
}

func args(values ...any) []object.Object {
	out := make([]object.Object, len(values))
	for i, v := range values {
		switch v := v.(type) {
		case string:
			out[i] = object.String(v)
		case object.Object:
			out[i] = v
		default:
			panic(v)
		}
	}
	return out
}

func kw(name string, value object.Object) object.KwArg { return object.KwArg{Name: name, Value: value} }

// wantErr checks err's kind and message and returns it.
func wantErr(t *testing.T, err error, kind *object.Error, msg string) *object.Error {
	t.Helper()
	var e *object.Error
	if err == nil || !errors.As(err, &e) || !errors.Is(err, kind) {
		t.Fatalf("error = %v, want %s", err, kind.Value)
	}
	if msg != "" && err.Error() != msg {
		t.Fatalf("message = %q, want %q", err.Error(), msg)
	}
	return e
}

func field(t *testing.T, e *object.Error, name string) object.Object {
	t.Helper()
	v, ok := e.Field(name)
	if !ok {
		t.Fatalf("error %q carries no %s", e, name)
	}
	return v
}

func pathField(t *testing.T, e *object.Error) string {
	t.Helper()
	return field(t, e, "path").(*pathvalue.Path).FilePath()
}

func memberName(t *testing.T, e *object.Error) string {
	t.Helper()
	m, ok := field(t, e, "member").(*archive.Member)
	if !ok {
		t.Fatalf("member data of %q is not a Member", e)
	}
	return m.Display
}

// entry is one member of an archive built with Go's writer.
type entry struct {
	name string
	body string
	mode stdfs.FileMode
	// raw writes the header as given, with declared as its size.
	raw      bool
	declared uint64
	flags    uint16
	method   uint16
}

func buildZip(t *testing.T, path string, entries ...entry) {
	t.Helper()
	var buf bytes.Buffer
	zw := gozip.NewWriter(&buf)
	for _, e := range entries {
		h := &gozip.FileHeader{Name: e.name, Method: e.method, Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
		h.Flags = e.flags
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h.SetMode(mode)
		if e.raw {
			h.CompressedSize64 = uint64(len(e.body))
			h.UncompressedSize64 = e.declared
			h.CRC32 = crc32.ChecksumIEEE([]byte(e.body))
			w, err := zw.CreateRaw(h)
			if err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(e.body))
			continue
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, reader object.Object) []string {
	t.Helper()
	items, err := object.Collect(reader)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.(*archive.Member).Display
	}
	return out
}

// listTree lists every path below dir, relative and sorted.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(dir, func(p string, d stdfs.DirEntry, err error) error {
		if err != nil || p == dir {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

func umask() os.FileMode {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return os.FileMode(old)
}

func TestExports(t *testing.T) {
	m := module(t)
	if m.Members["Member"] != archive.MemberType.Function || m.Members["UnsafeError"] != archive.UnsafeError {
		t.Fatal("zip does not export the shared Member and UnsafeError")
	}
	_, err := m.Members["Reader"].(*object.Function).Call(object.CallArgs{})
	wantErr(t, err, object.TypeError, "Reader cannot be constructed; use zip.open(path)")
	_, err = m.Members["Writer"].(*object.Function).Call(object.CallArgs{})
	wantErr(t, err, object.TypeError, "Writer cannot be constructed; use zip.create(path)")
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	os.MkdirAll(filepath.Join(src, "sub", "nested"), 0o755)
	os.MkdirAll(filepath.Join(src, "empty"), 0o755)
	os.WriteFile(filepath.Join(src, "sub", "nested", "file.txt"), []byte("deep\n"), 0o644)
	os.WriteFile(filepath.Join(src, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.Chmod(filepath.Join(src, "run.sh"), 0o755)
	os.Symlink("run.sh", filepath.Join(src, "link"))
	stamp := time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)
	os.Chtimes(filepath.Join(src, "run.sh"), stamp, stamp)

	path := filepath.Join(dir, "out.zip")
	w := mustCall(t, "create", args(path))
	if w.(*Writer).String() != "<zip.Writer "+path+">" {
		t.Fatalf("shows as %v", w)
	}
	mustInvoke(t, w, "add", args(src))
	mustInvoke(t, w, "add_data", args("gen/data.txt", object.NewBytes([]byte("made"))), kw("mode", object.Integer(0o600)))
	mustInvoke(t, w, "close", nil)
	mustInvoke(t, w, "close", nil)

	r := mustCall(t, "open", args(path))
	want := []string{"src", "src/empty", "src/link", "src/run.sh", "src/sub", "src/sub/nested", "src/sub/nested/file.txt", "gen/data.txt"}
	if got := names(t, r); !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %v", got)
	}
	// A second pass starts from the first member again.
	if got := names(t, r); !reflect.DeepEqual(got, want) {
		t.Fatalf("second pass = %v", got)
	}
	link := mustInvoke(t, r, "find", args("src/link")).(*archive.Member)
	if target, _ := link.GetAttr("link_target"); target != object.String("run.sh") {
		t.Fatalf("link_target = %v", target)
	}
	run := mustInvoke(t, r, "find", args("src/run.sh")).(*archive.Member)
	if run.Mode != 0o755 || !run.ModTime.Equal(stamp) {
		t.Fatalf("run.sh mode %o time %v", run.Mode, run.ModTime)
	}
	// The time is in the local zone, as fs.stat and tar give it, so its
	// zone is a name zone= accepts.
	if run.ModTime.Location() != time.Local {
		t.Fatalf("run.sh time is in %q, not the local zone", run.ModTime.Location())
	}
	mustInvoke(t, r, "close", nil)

	dst := filepath.Join(dir, "dst")
	if v := mustCall(t, "extract", args(path, dst)); v != object.Nil {
		t.Fatalf("extract returned %v", v)
	}
	data, _ := os.ReadFile(filepath.Join(dst, "src", "sub", "nested", "file.txt"))
	if string(data) != "deep\n" {
		t.Fatalf("content %q", data)
	}
	info, _ := os.Stat(filepath.Join(dst, "src", "run.sh"))
	if info.Mode().Perm() != 0o755&^umask() || !info.ModTime().Equal(stamp) {
		t.Fatalf("run.sh mode %v time %v", info.Mode(), info.ModTime())
	}
	if info, _ := os.Stat(filepath.Join(dst, "gen", "data.txt")); info.Mode().Perm() != 0o600&^umask() {
		t.Fatalf("data.txt mode %v", info.Mode())
	}
	if target, err := os.Readlink(filepath.Join(dst, "src", "link")); err != nil || target != "run.sh" {
		t.Fatalf("link = %q, %v", target, err)
	}
	if info, err := os.Stat(filepath.Join(dst, "src", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty dir: %v", err)
	}
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		label   string
		entries []entry
		kws     []object.KwArg
		msg     string // with ARCH and DST standing for the paths
		member  string // "*" for a refusal of the whole archive
		created bool   // whether dst exists afterwards
	}{
		{"dotdot", []entry{{name: "ok.txt", body: "x"}, {name: "../x", body: "evil"}}, nil,
			`extract() member "../x" of "ARCH" would be written outside "DST"`, "../x", false},
		{"absolute", []entry{{name: "/abs", body: "evil"}}, nil,
			`extract() member "/abs" of "ARCH" would be written outside "DST"`, "/abs", false},
		{"backslash", []entry{{name: `a\b`, body: "evil"}}, nil,
			`extract() member "a\b" of "ARCH" would be written outside "DST"`, `a\b`, false},
		{"empty", []entry{{name: "", body: "evil"}}, nil,
			`extract() member "" of "ARCH" would be written outside "DST"`, "", false},
		{"device", []entry{{name: "dev/null", mode: stdfs.ModeDevice | stdfs.ModeCharDevice | 0o666}}, nil,
			`extract() member "dev/null" of "ARCH" is a device`, "dev/null", false},
		{"link absolute", []entry{{name: "current", body: "/etc", mode: stdfs.ModeSymlink | 0o777}}, nil,
			`extract() link "current" of "ARCH" points outside "DST": "/etc"`, "current", true},
		{"link up", []entry{{name: "lib/up", body: "../../x", mode: stdfs.ModeSymlink | 0o777}}, nil,
			`extract() link "lib/up" of "ARCH" points outside "DST": "../../x"`, "lib/up", true},
		{"link escapes through inside dir", []entry{{name: "sub/", mode: stdfs.ModeDir | 0o755}, {name: "esc", body: "sub/../..", mode: stdfs.ModeSymlink | 0o777}}, nil,
			`extract() link "esc" of "ARCH" points outside "DST": "sub/../.."`, "esc", true},
		{"oversize", []entry{{name: "a.bin", body: "0123456789", raw: true, declared: 4}}, nil,
			`extract() member "a.bin" of "ARCH" is larger than the 4 bytes it declares`, "a.bin", true},
		{"max_size", []entry{{name: "a", body: "12345"}, {name: "b", body: "123456"}}, []object.KwArg{kw("max_size", object.Integer(10))},
			`extract() "ARCH" unpacks to 11 bytes, over max_size 10`, "*", false},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			dir := t.TempDir()
			arch := filepath.Join(dir, "a.zip")
			dst := filepath.Join(dir, "dst")
			buildZip(t, arch, c.entries...)
			_, err := call(t, "extract", args(arch, dst), c.kws...)
			msg := strings.NewReplacer("ARCH", arch, "DST", dst).Replace(c.msg)
			e := wantErr(t, err, archive.UnsafeError, msg)
			if c.member == "*" {
				if field(t, e, "member") != object.Nil {
					t.Fatalf("member = %v, want nil", field(t, e, "member"))
				}
			} else if got := memberName(t, e); got != c.member {
				t.Fatalf("member = %q", got)
			}
			if pathField(t, e) != arch {
				t.Fatalf("path = %q", pathField(t, e))
			}
			// Nothing lands beside dst, and only what was allowed lands in it.
			outside := listTree(t, dir)
			var stray []string
			for _, p := range outside {
				if p != "a.zip" && p != "dst" && !strings.HasPrefix(p, "dst/") {
					stray = append(stray, p)
				}
			}
			if len(stray) > 0 {
				t.Fatalf("written outside dst: %v", stray)
			}
			if _, statErr := os.Stat(dst); (statErr == nil) != c.created {
				t.Fatalf("dst exists = %v, want %v", statErr == nil, c.created)
			}
			if c.label == "oversize" {
				if _, err := os.Lstat(filepath.Join(dst, "a.bin")); err == nil {
					t.Fatal("the oversized member was left on disk")
				}
			}
		})
	}
}

func TestLinksInsideAndOutside(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "a.zip")
	// A link inside dst, then a member written through it: allowed, and it
	// lands inside.
	buildZip(t, arch,
		entry{name: "sub/", mode: stdfs.ModeDir | 0o755},
		entry{name: "inside", body: "sub", mode: stdfs.ModeSymlink | 0o777},
		entry{name: "inside/x.txt", body: "through"})
	dst := filepath.Join(dir, "dst")
	mustCall(t, "extract", args(arch, dst))
	if data, _ := os.ReadFile(filepath.Join(dst, "sub", "x.txt")); string(data) != "through" {
		t.Fatalf("sub/x.txt = %q", data)
	}

	// A link already in dst that leads outside: the operating system
	// interface refuses to write through it.
	elsewhere := filepath.Join(dir, "elsewhere")
	os.Mkdir(elsewhere, 0o755)
	dst2 := filepath.Join(dir, "dst2")
	os.Mkdir(dst2, 0o755)
	os.Symlink(elsewhere, filepath.Join(dst2, "out"))
	buildZip(t, arch, entry{name: "out/x", body: "evil"})
	_, err := call(t, "extract", args(arch, dst2))
	e := wantErr(t, err, archive.UnsafeError, `extract() member "out/x" of "`+arch+`" would be written outside "`+dst2+`"`)
	if memberName(t, e) != "out/x" {
		t.Fatal("wrong member")
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("written through the link: %v", entries)
	}
}

func TestBadLastMemberLeavesDstEmpty(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "a.zip")
	buildZip(t, arch, entry{name: "a.txt", body: "a"}, entry{name: "b.txt", body: "b"}, entry{name: "../c", body: "c"})
	dst := filepath.Join(dir, "dst")
	_, err := call(t, "extract", args(arch, dst))
	wantErr(t, err, archive.UnsafeError, "")
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("dst was touched: %v", err)
	}
	// An existing dst stays as it was.
	os.Mkdir(dst, 0o755)
	call(t, "extract", args(arch, dst))
	if got := listTree(t, dst); len(got) != 0 {
		t.Fatalf("dst = %v", got)
	}
}

func TestOverwrite(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "a.zip")
	dst := filepath.Join(dir, "app")
	buildZip(t, arch, entry{name: "bin/tool", body: "v1"})
	mustCall(t, "extract", args(arch, dst))
	buildZip(t, arch, entry{name: "bin/tool", body: "v2"})
	_, err := call(t, "extract", args(arch, dst))
	tool := filepath.Join(dst, "bin", "tool")
	e := wantErr(t, err, object.ExistError, `extract() "`+tool+`" already exists (pass overwrite=true)`)
	if pathField(t, e) != tool {
		t.Fatalf("path = %q", pathField(t, e))
	}
	mustCall(t, "extract", args(arch, dst), kw("overwrite", object.True))
	if data, _ := os.ReadFile(tool); string(data) != "v2" {
		t.Fatalf("tool = %q", data)
	}

	// A directory where a file must go raises with or without overwrite.
	buildZip(t, arch, entry{name: "bin", body: "file"})
	for _, overwrite := range []object.Object{object.False, object.True} {
		_, err := call(t, "extract", args(arch, dst), kw("overwrite", overwrite))
		e := wantErr(t, err, object.ExistError, `extract() "`+filepath.Join(dst, "bin")+`" is a directory`)
		if pathField(t, e) != filepath.Join(dst, "bin") {
			t.Fatal("wrong path")
		}
	}
	// A file where a directory must go.
	buildZip(t, arch, entry{name: "bin/tool/x", body: "x"})
	_, err = call(t, "extract", args(arch, dst), kw("overwrite", object.True))
	wantErr(t, err, object.ExistError, `extract() "`+tool+`" is not a directory`)

	// An existing link at the name is replaced, and its target untouched.
	target := filepath.Join(dir, "precious.txt")
	os.WriteFile(target, []byte("keep"), 0o644)
	os.Symlink(target, filepath.Join(dst, "link.txt"))
	buildZip(t, arch, entry{name: "link.txt", body: "new"})
	_, err = call(t, "extract", args(arch, dst))
	wantErr(t, err, object.ExistError, "")
	mustCall(t, "extract", args(arch, dst), kw("overwrite", object.True))
	if data, _ := os.ReadFile(target); string(data) != "keep" {
		t.Fatalf("link target changed: %q", data)
	}
	info, _ := os.Lstat(filepath.Join(dst, "link.txt"))
	if !info.Mode().IsRegular() {
		t.Fatalf("link.txt is %v", info.Mode())
	}

	// The same name twice: the second is a file already there, and
	// overwrite keeps the last.
	buildZip(t, arch, entry{name: "twice", body: "first"}, entry{name: "twice", body: "second"})
	dst2 := filepath.Join(dir, "two")
	_, err = call(t, "extract", args(arch, dst2))
	wantErr(t, err, object.ExistError, "")
	mustCall(t, "extract", args(arch, dst2), kw("overwrite", object.True))
	if data, _ := os.ReadFile(filepath.Join(dst2, "twice")); string(data) != "second" {
		t.Fatalf("twice = %q", data)
	}
}

func TestSetIDBitsDropped(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "a.zip")
	buildZip(t, arch, entry{name: "tool", body: "x", mode: stdfs.ModeSetuid | stdfs.ModeSetgid | stdfs.ModeSticky | 0o755})
	dst := filepath.Join(dir, "dst")
	mustCall(t, "extract", args(arch, dst))
	info, _ := os.Stat(filepath.Join(dst, "tool"))
	if info.Mode()&(stdfs.ModeSetuid|stdfs.ModeSetgid|stdfs.ModeSticky) != 0 {
		t.Fatalf("mode = %v", info.Mode())
	}
}

func TestReader(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "bundle.zip")
	buildZip(t, arch,
		entry{name: "docs/", mode: stdfs.ModeDir | 0o755},
		entry{name: "a.txt", body: "first"},
		entry{name: "a.txt", body: "second!"},
		entry{name: "manifest.json", body: `{"v": 1}`, method: gozip.Deflate})
	r := mustCall(t, "open", args(arch))
	if r.(*Reader).String() != "<zip.Reader "+arch+">" {
		t.Fatalf("shows as %v", r)
	}
	if v := mustInvoke(t, r, "find", args("missing")); v != object.Nil {
		t.Fatalf("find missing = %v", v)
	}
	last := mustInvoke(t, r, "find", args("a.txt")).(*archive.Member)
	if last.Size != 7 {
		t.Fatalf("duplicate resolved to size %d", last.Size)
	}
	_, err := invoke(t, r, "open", args("missing"))
	wantErr(t, err, object.KeyError, `open() no member "missing" in "`+arch+`"`)
	_, err = invoke(t, r, "open", args("docs"))
	wantErr(t, err, object.ValueError, `open() member "docs" is a directory`)
	_, err = invoke(t, r, "open", args(object.Integer(1)))
	wantErr(t, err, object.TypeError, "open() argument 'member' must be Member or String, got Int")

	file := mustInvoke(t, r, "open", args("manifest.json"))
	if s, _ := file.ToString(); s != "<File "+arch+":manifest.json>" {
		t.Fatalf("member File shows as %s", s)
	}
	if p, _ := file.GetAttr("path"); p != object.Nil {
		t.Fatalf("member File path = %v", p)
	}
	if got := mustInvoke(t, file, "read", args(object.Integer(3))); string(got.(object.Bytes)) != `{"v` {
		t.Fatalf("read(3) = %v", got)
	}

	// A Member of another archive.
	other := mustCall(t, "open", args(arch))
	foreign := mustInvoke(t, other, "find", args("a.txt"))
	_, err = invoke(t, r, "open", args(foreign))
	wantErr(t, err, object.ValueError, `open() member "a.txt" is not from "`+arch+`"`)
	mustInvoke(t, other, "close", nil)

	if closed, _ := file.GetAttr("closed"); closed != object.False {
		t.Fatal("a member File is closed while its archive is open")
	}
	mustInvoke(t, r, "close", nil)
	mustInvoke(t, r, "close", nil)
	// The member can no longer be read, and its File says so.
	if closed, _ := file.GetAttr("closed"); closed != object.True {
		t.Fatal("a member File of a closed archive is not closed")
	}
	_, err = invoke(t, file, "read", nil)
	wantErr(t, err, object.ValueError, `read() member "manifest.json" is no longer open`)
	_, err = object.Collect(r)
	wantErr(t, err, object.ValueError, "cannot iterate a closed archive")
	_, err = invoke(t, r, "find", args("a.txt"))
	wantErr(t, err, object.ValueError, "")
}

func TestDamage(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "bundle.zip")
	buildZip(t, arch, entry{name: "x", body: "hello, archive"}, entry{name: "secret", body: "x", flags: 0x1})
	raw, _ := os.ReadFile(arch)
	i := bytes.Index(raw, []byte("hello, archive"))
	raw[i] = 'j'
	os.WriteFile(arch, raw, 0o644)

	r := mustCall(t, "open", args(arch))
	defer invoke(t, r, "close", nil)
	file := mustInvoke(t, r, "open", args("x"))
	_, err := invoke(t, file, "read", nil)
	e := wantErr(t, err, object.ParseError, `read() corrupt data in member "x" of "`+arch+`"`)
	if pathField(t, e) != arch {
		t.Fatal("wrong path")
	}
	_, err = invoke(t, r, "open", args("secret"))
	wantErr(t, err, object.ParseError, `open() member "secret" of "`+arch+`" is encrypted`)

	_, err = call(t, "extract", args(arch, filepath.Join(dir, "dst")))
	wantErr(t, err, object.ParseError, `extract() corrupt data in member "x" of "`+arch+`"`)

	notZip := filepath.Join(dir, "x.zip")
	os.WriteFile(notZip, []byte("plain text"), 0o644)
	_, err = call(t, "open", args(notZip))
	e = wantErr(t, err, object.ParseError, `open() "`+notZip+`" is not a zip archive`)
	if pathField(t, e) != notZip {
		t.Fatal("wrong path")
	}
	_, err = call(t, "open", args(filepath.Join(dir, "missing.zip")))
	wantErr(t, err, object.NotExistError, "")
	// A directory is a file that cannot be read, as gzip and tar say, and
	// an IOError of no more specific kind.
	_, err = call(t, "open", args(dir))
	e = wantErr(t, err, object.IOError, `open() cannot read "`+dir+`": is a directory`)
	if errors.Is(err, object.ParseError) || errors.Is(err, object.NetworkError) {
		t.Fatalf("a directory is %v", err)
	}
	if pathField(t, e) != dir {
		t.Fatal("wrong path")
	}
}

func TestWriterErrors(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "out.zip")
	w := mustCall(t, "create", args(arch))
	mustInvoke(t, w, "add_data", args("a.txt", "x"))
	_, err := invoke(t, w, "add_data", args("a.txt", "y"))
	wantErr(t, err, object.ValueError, `add_data() "a.txt" is already in the archive`)
	_, err = invoke(t, w, "add_data", args("../x", "y"))
	wantErr(t, err, object.ValueError, `add_data() name "../x" would be unpacked outside its directory`)
	_, err = invoke(t, w, "add_data", args("m", "y"), kw("mode", object.Integer(0o1000)))
	wantErr(t, err, object.ValueError, "add_data() argument 'mode' must be between 0 and 0o777, got 0o1000")
	_, err = invoke(t, w, "add_data", args("m", "y"), kw("mode", object.Integer(-1)))
	wantErr(t, err, object.ValueError, "add_data() argument 'mode' must be between 0 and 0o777, got -0o1")
	_, err = invoke(t, w, "add", args("."))
	wantErr(t, err, object.ValueError, `add() "." has no name; pass name=`)
	_, err = invoke(t, w, "add", args(dir), kw("name", object.String("/abs")))
	wantErr(t, err, object.ValueError, `add() name "/abs" would be unpacked outside its directory`)
	_, err = invoke(t, w, "add", args(filepath.Join(dir, "missing")))
	wantErr(t, err, object.NotExistError, "")
	src := filepath.Join(dir, "src")
	os.Mkdir(src, 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	_, err = invoke(t, w, "add", args(filepath.Join(src, "a.txt")))
	wantErr(t, err, object.ValueError, `add() "a.txt" is already in the archive`)
	mustInvoke(t, w, "close", nil)
	_, err = invoke(t, w, "add_data", args("b.txt", "x"))
	wantErr(t, err, object.ValueError, "add_data() called on closed archive")

	// The archive's own file is passed over when it lies inside the added
	// directory.
	own := filepath.Join(src, "self.zip")
	w = mustCall(t, "create", args(own))
	mustInvoke(t, w, "add", args(src), kw("name", object.String("top")))
	mustInvoke(t, w, "close", nil)
	r := mustCall(t, "open", args(own))
	if got := names(t, r); !reflect.DeepEqual(got, []string{"top", "top/a.txt"}) {
		t.Fatalf("members = %v", got)
	}
	mustInvoke(t, r, "close", nil)

	// An archive that was never closed has no index.
	unfinished := filepath.Join(dir, "unfinished.zip")
	w = mustCall(t, "create", args(unfinished))
	mustInvoke(t, w, "add_data", args("a.txt", strings.Repeat("x", 100000)))
	_, err = call(t, "open", args(unfinished))
	wantErr(t, err, object.ParseError, `open() "`+unfinished+`" is not a zip archive`)
	mustInvoke(t, w, "close", nil)
}

func TestMethodExtract(t *testing.T) {
	dir := t.TempDir()
	arch := filepath.Join(dir, "a.zip")
	buildZip(t, arch, entry{name: "project/docs/a.md", body: "doc"}, entry{name: "../evil", body: "x"})
	r := mustCall(t, "open", args(arch))
	defer invoke(t, r, "close", nil)
	dst := filepath.Join(dir, "site")
	written := mustInvoke(t, r, "extract", args("project/docs/a.md", dst), kw("name", object.String("a.md")))
	if written.(*pathvalue.Path).FilePath() != filepath.Join(dst, "a.md") {
		t.Fatalf("returned %v", written)
	}
	// A bad stored name can be unpacked under a name the script computed.
	mustInvoke(t, r, "extract", args("../evil", dst), kw("name", pathvalue.NewPath("evil")))
	_, err := invoke(t, r, "extract", args("project/docs/a.md", dst), kw("name", object.String("../a.md")))
	wantErr(t, err, archive.UnsafeError, `extract() member "project/docs/a.md" of "`+arch+`" would be written outside "`+dst+`" as "../a.md"`)
	_, err = invoke(t, r, "extract", args("nope", dst))
	wantErr(t, err, object.KeyError, `extract() no member "nope" in "`+arch+`"`)
	if got := listTree(t, dst); !reflect.DeepEqual(got, []string{"a.md", "evil"}) {
		t.Fatalf("dst = %v", got)
	}
}
