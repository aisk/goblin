package tar

import (
	gotar "archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
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

var stamp = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// file, dir and link make headers for tarBytes.
func file(name, body string) *gotar.Header {
	return &gotar.Header{Typeflag: gotar.TypeReg, Name: name, Size: int64(len(body)), Mode: 0o644, ModTime: stamp, Linkname: body}
}

func dir(name string) *gotar.Header {
	return &gotar.Header{Typeflag: gotar.TypeDir, Name: name + "/", Mode: 0o755, ModTime: stamp}
}

func link(name, target string) *gotar.Header {
	return &gotar.Header{Typeflag: gotar.TypeSymlink, Name: name, Linkname: target, Mode: 0o777, ModTime: stamp}
}

// tarBytes writes headers with Go's writer; a regular file's body is
// carried in its Linkname field.
func tarBytes(t *testing.T, headers ...*gotar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := gotar.NewWriter(&buf)
	for _, h := range headers {
		body := ""
		if h.Typeflag == gotar.TypeReg {
			body, h.Linkname = h.Linkname, ""
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		io.WriteString(tw, body)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gzipped(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(data)
	zw.Close()
	return buf.Bytes()
}

func write(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
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

func listTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d stdfs.DirEntry, err error) error {
		if err != nil || p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
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

// stream is a Goblin value with read(size) and close() over data, as a
// user type would be: it records whether close() was called.
func stream(data []byte, closed *bool) object.Object {
	r := bytes.NewReader(data)
	return &object.Module{Name: "stream", Members: map[string]object.Object{
		"read": &object.Function{Name: "read", Fn: func(a object.CallArgs) (object.Object, error) {
			buf := make([]byte, int(a.Positional[0].(object.Integer)))
			n, _ := r.Read(buf)
			return object.NewBytes(buf[:n]), nil
		}},
		"close": &object.Function{Name: "close", Fn: func(object.CallArgs) (object.Object, error) {
			*closed = true
			return object.Nil, nil
		}},
	}}
}

// sinkValue is a Goblin value with write(data) and close().
func sinkValue(buf *bytes.Buffer, closed *bool) object.Object {
	return &object.Module{Name: "sink", Members: map[string]object.Object{
		"write": &object.Function{Name: "write", Fn: func(a object.CallArgs) (object.Object, error) {
			chunk := a.Positional[0].(object.Bytes)
			buf.Write(chunk)
			return object.Integer(len(chunk)), nil
		}},
		"close": &object.Function{Name: "close", Fn: func(object.CallArgs) (object.Object, error) {
			*closed = true
			return object.Nil, nil
		}},
	}}
}

func TestExports(t *testing.T) {
	m := module(t)
	if m.Members["Member"] != archive.MemberType.Function || m.Members["UnsafeError"] != archive.UnsafeError {
		t.Fatal("tar does not export the shared Member and UnsafeError")
	}
	_, err := call(t, "Reader", args("x"))
	wantErr(t, err, object.TypeError, "Reader() argument 'source' must have a read(size) method, got String")
	_, err = call(t, "Writer", args("x"))
	wantErr(t, err, object.TypeError, "Writer() argument 'target' must have a write(data) method, got String")
}

func TestRoundTrip(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	os.MkdirAll(filepath.Join(src, "sub", "nested"), 0o755)
	os.MkdirAll(filepath.Join(src, "empty"), 0o755)
	os.WriteFile(filepath.Join(src, "sub", "nested", "file.txt"), []byte("deep\n"), 0o644)
	os.WriteFile(filepath.Join(src, "run.sh"), []byte("#!/bin/sh\n"), 0o755)
	os.Chmod(filepath.Join(src, "run.sh"), 0o755)
	os.Symlink("run.sh", filepath.Join(src, "link"))
	os.Chtimes(filepath.Join(src, "run.sh"), stamp, stamp)

	for _, compressed := range []bool{false, true} {
		path := filepath.Join(root, "out.tar")
		w := mustCall(t, "create", args(path), kw("gzip", object.Bool(compressed)))
		if w.(*Writer).String() != "<tar.Writer "+path+">" {
			t.Fatalf("shows as %v", w)
		}
		mustInvoke(t, w, "add", args(src))
		mustInvoke(t, w, "add_data", args("gen/data.txt", "made"), kw("mode", object.Integer(0o600)))
		mustInvoke(t, w, "close", nil)
		mustInvoke(t, w, "close", nil)

		raw, _ := os.ReadFile(path)
		if compressed != bytes.HasPrefix(raw, []byte{0x1f, 0x8b}) {
			t.Fatalf("gzip=%v wrote % x", compressed, raw[:2])
		}
		r := mustCall(t, "open", args(path))
		if r.(*Reader).String() != "<tar.Reader "+path+">" {
			t.Fatalf("shows as %v", r)
		}
		want := []string{"src", "src/empty", "src/link", "src/run.sh", "src/sub", "src/sub/nested", "src/sub/nested/file.txt", "gen/data.txt"}
		if got := names(t, r); !reflect.DeepEqual(got, want) {
			t.Fatalf("members = %v", got)
		}
		mustInvoke(t, r, "close", nil)

		// The owner is not recorded.
		var input io.Reader = bytes.NewReader(raw)
		if compressed {
			input, _ = gzip.NewReader(input)
		}
		tr := gotar.NewReader(input)
		for {
			h, err := tr.Next()
			if err != nil {
				break
			}
			if h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" {
				t.Fatalf("%s records its owner: %d %d %q %q", h.Name, h.Uid, h.Gid, h.Uname, h.Gname)
			}
		}

		dst := filepath.Join(root, "dst")
		os.RemoveAll(dst)
		mustCall(t, "extract", args(path, dst))
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
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		label   string
		headers []*gotar.Header
		kws     []object.KwArg
		msg     string
		member  string
		kept    []string // what stays in dst
	}{
		{"dotdot", []*gotar.Header{file("ok.txt", "x"), file("../x", "evil")}, nil,
			`extract() member "../x" of "ARCH" would be written outside "DST"`, "../x", []string{"ok.txt"}},
		{"absolute", []*gotar.Header{file("/abs", "evil")}, nil,
			`extract() member "/abs" of "ARCH" would be written outside "DST"`, "/abs", nil},
		{"backslash", []*gotar.Header{file(`a\b`, "evil")}, nil,
			`extract() member "a\b" of "ARCH" would be written outside "DST"`, `a\b`, nil},
		{"empty", []*gotar.Header{file("", "evil")}, nil,
			`extract() member "" of "ARCH" would be written outside "DST"`, "", nil},
		{"device", []*gotar.Header{{Typeflag: gotar.TypeChar, Name: "dev/null", Mode: 0o666, Devmajor: 1, Devminor: 3}}, nil,
			`extract() member "dev/null" of "ARCH" is a device`, "dev/null", nil},
		{"fifo", []*gotar.Header{{Typeflag: gotar.TypeFifo, Name: "pipe", Mode: 0o666}}, nil,
			`extract() member "pipe" of "ARCH" is a FIFO`, "pipe", nil},
		{"link absolute", []*gotar.Header{link("current", "/etc")}, nil,
			`extract() link "current" of "ARCH" points outside "DST": "/etc"`, "current", nil},
		{"link up", []*gotar.Header{link("lib/up", "../../x")}, nil,
			`extract() link "lib/up" of "ARCH" points outside "DST": "../../x"`, "lib/up", nil},
		{"hard link up", []*gotar.Header{{Typeflag: gotar.TypeLink, Name: "hard", Linkname: "../x"}}, nil,
			`extract() link "hard" of "ARCH" points outside "DST": "../x"`, "hard", nil},
		{"max_size", []*gotar.Header{file("a", "12345"), file("b", "123456"), file("c", "1")}, []object.KwArg{kw("max_size", object.Integer(10))},
			`extract() "ARCH" unpacks to at least 11 bytes, over max_size 10`, "b", []string{"a"}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			root := t.TempDir()
			arch := write(t, filepath.Join(root, "a.tar"), tarBytes(t, c.headers...))
			dst := filepath.Join(root, "dst")
			_, err := call(t, "extract", args(arch, dst), c.kws...)
			msg := strings.NewReplacer("ARCH", arch, "DST", dst).Replace(c.msg)
			e := wantErr(t, err, archive.UnsafeError, msg)
			if got := memberName(t, e); got != c.member {
				t.Fatalf("member = %q", got)
			}
			if pathField(t, e) != arch {
				t.Fatalf("path = %q", pathField(t, e))
			}
			for _, p := range listTree(t, root) {
				if p != "a.tar" && p != "dst" && !strings.HasPrefix(p, "dst/") {
					t.Fatalf("written outside dst: %s", p)
				}
			}
			if got := listTree(t, dst); !reflect.DeepEqual(got, c.kept) {
				t.Fatalf("dst = %v, want %v", got, c.kept)
			}
		})
	}
}

func TestLinks(t *testing.T) {
	root := t.TempDir()
	arch := write(t, filepath.Join(root, "a.tar"), tarBytes(t,
		dir("sub"),
		link("inside", "sub"),
		file("inside/x.txt", "through"),
		file("a.txt", "shared"),
		&gotar.Header{Typeflag: gotar.TypeLink, Name: "hard", Linkname: "a.txt"}))
	dst := filepath.Join(root, "dst")
	mustCall(t, "extract", args(arch, dst))
	if data, _ := os.ReadFile(filepath.Join(dst, "sub", "x.txt")); string(data) != "through" {
		t.Fatalf("sub/x.txt = %q", data)
	}
	a, _ := os.Stat(filepath.Join(dst, "a.txt"))
	hard, _ := os.Stat(filepath.Join(dst, "hard"))
	if !os.SameFile(a, hard) {
		t.Fatal("hard is not a hard link to a.txt")
	}

	// A hard link member: link_target set, every is_ property false.
	r := mustCall(t, "open", args(arch))
	defer invoke(t, r, "close", nil)
	items, _ := object.Collect(r)
	h := items[4].(*archive.Member)
	for _, prop := range []string{"is_file", "is_dir", "is_symlink"} {
		if v, _ := h.GetAttr(prop); v != object.False {
			t.Fatalf("hard link %s = %v", prop, v)
		}
	}
	if v, _ := h.GetAttr("link_target"); v != object.String("a.txt") {
		t.Fatalf("link_target = %v", v)
	}

	// A link already in dst leading outside.
	elsewhere := filepath.Join(root, "elsewhere")
	os.Mkdir(elsewhere, 0o755)
	dst2 := filepath.Join(root, "dst2")
	os.Mkdir(dst2, 0o755)
	os.Symlink(elsewhere, filepath.Join(dst2, "out"))
	arch2 := write(t, filepath.Join(root, "b.tar"), tarBytes(t, file("out/x", "evil")))
	_, err := call(t, "extract", args(arch2, dst2))
	wantErr(t, err, archive.UnsafeError, `extract() member "out/x" of "`+arch2+`" would be written outside "`+dst2+`"`)
	// A link from the archive itself leading outside, then a member through
	// it: the link is refused first.
	arch3 := write(t, filepath.Join(root, "c.tar"), tarBytes(t, link("esc", "../elsewhere"), file("esc/x", "evil")))
	_, err = call(t, "extract", args(arch3, filepath.Join(root, "dst3")))
	wantErr(t, err, archive.UnsafeError, "")
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("written through a link: %v", entries)
	}
}

func TestOverwriteAndSetID(t *testing.T) {
	root := t.TempDir()
	dst := filepath.Join(root, "app")
	arch := write(t, filepath.Join(root, "a.tar"), tarBytes(t, file("bin/tool", "v1")))
	mustCall(t, "extract", args(arch, dst))
	_, err := call(t, "extract", args(arch, dst))
	tool := filepath.Join(dst, "bin", "tool")
	e := wantErr(t, err, object.ExistError, `extract() "`+tool+`" already exists (pass overwrite=true)`)
	if pathField(t, e) != tool {
		t.Fatal("wrong path")
	}
	write(t, arch, tarBytes(t, file("bin/tool", "v2")))
	mustCall(t, "extract", args(arch, dst), kw("overwrite", object.True))
	if data, _ := os.ReadFile(tool); string(data) != "v2" {
		t.Fatalf("tool = %q", data)
	}
	write(t, arch, tarBytes(t, file("bin", "x")))
	_, err = call(t, "extract", args(arch, dst), kw("overwrite", object.True))
	wantErr(t, err, object.ExistError, `extract() "`+filepath.Join(dst, "bin")+`" is a directory`)

	setid := file("tool", "x")
	setid.Mode = 0o7755
	write(t, arch, tarBytes(t, setid))
	mustCall(t, "extract", args(arch, filepath.Join(root, "s")))
	info, _ := os.Stat(filepath.Join(root, "s", "tool"))
	if info.Mode()&(stdfs.ModeSetuid|stdfs.ModeSetgid|stdfs.ModeSticky) != 0 {
		t.Fatalf("mode = %v", info.Mode())
	}
}

func TestReaderOrder(t *testing.T) {
	root := t.TempDir()
	data := tarBytes(t, file("a.log", "aaaa\nERROR one\n"), file("b.log", "bbbb"), dir("d"), file("c.log", "cccc"))
	for _, variant := range []struct {
		label string
		data  []byte
	}{
		{"plain", data},
		{"gzip", gzipped(t, data)},
		// Two gzip streams one after another read as one.
		{"two gzip streams", append(gzipped(t, data[:1024]), gzipped(t, data[1024:])...)},
	} {
		t.Run(variant.label, func(t *testing.T) {
			path := write(t, filepath.Join(root, "x.tar"), variant.data)
			r := mustCall(t, "open", args(path))
			defer invoke(t, r, "close", nil)
			it, err := object.Iterate(r)
			if err != nil {
				t.Fatal(err)
			}
			first, _, _ := it.Next()
			it.Close()
			f := mustInvoke(t, r, "open", args(first))
			if got := mustInvoke(t, f, "read", args(object.Integer(5))); string(got.(object.Bytes)) != "aaaa\n" {
				t.Fatalf("read(5) = %v", got)
			}
			// A second File continues where the first stopped.
			f2 := mustInvoke(t, r, "open", args(first))
			if rest := mustInvoke(t, f2, "read", nil); string(rest.(object.Bytes)) != "ERROR one\n" {
				t.Fatalf("rest = %v", rest)
			}
			_, err = invoke(t, r, "extract", args(first, filepath.Join(root, "out")))
			wantErr(t, err, object.ValueError, `extract() member "a.log" was partly read through open(); a tar archive is read in order`)
			// A later loop continues after the member the first one stopped at,
			// passing over the unread content of b.log.
			if got := names(t, r); !reflect.DeepEqual(got, []string{"b.log", "d", "c.log"}) {
				t.Fatalf("rest = %v", got)
			}
			_, err = invoke(t, r, "open", args(first))
			wantErr(t, err, object.ValueError, `open() "a.log" is not the current member; a tar archive is read in order`)
			_, err = invoke(t, f, "read", nil)
			wantErr(t, err, object.ValueError, `read() member "a.log" is no longer open`)
			_, err = invoke(t, r, "open", args("c.log"))
			wantErr(t, err, object.TypeError, "open() argument 'member' must be Member, got String")
		})
	}
}

func TestDamage(t *testing.T) {
	root := t.TempDir()
	notTar := write(t, filepath.Join(root, "x.tar"), []byte(strings.Repeat("plain text\n", 100)))
	r := mustCall(t, "open", args(notTar))
	_, err := object.Collect(r)
	e := wantErr(t, err, object.ParseError, `"`+notTar+`" is not a tar archive`)
	if pathField(t, e) != notTar {
		t.Fatal("wrong path")
	}
	mustInvoke(t, r, "close", nil)
	_, err = call(t, "extract", args(notTar, filepath.Join(root, "dst")))
	wantErr(t, err, object.ParseError, `extract() "`+notTar+`" is not a tar archive`)

	// Damage after the first member.
	data := tarBytes(t, file("a", "aaaa"), file("b", "bbbb"))
	copy(data[1024:], bytes.Repeat([]byte("x"), 100))
	damaged := write(t, filepath.Join(root, "d.tar"), data)
	_, err = call(t, "extract", args(damaged, filepath.Join(root, "dst2")))
	wantErr(t, err, object.ParseError, `extract() corrupt tar data in "`+damaged+`"`)

	// A truncated member's content.
	short := write(t, filepath.Join(root, "s.tar"), tarBytes(t, file("big", strings.Repeat("z", 2000)))[:1024])
	r = mustCall(t, "open", args(short))
	it, _ := object.Iterate(r)
	m, _, _ := it.Next()
	f := mustInvoke(t, r, "open", args(m))
	_, err = invoke(t, f, "read", nil)
	wantErr(t, err, object.ParseError, `read() corrupt data in member "big" of "`+short+`"`)
	mustInvoke(t, r, "close", nil)

	for _, c := range []struct {
		name, magic, what string
	}{
		{"x.tar.xz", "\xfd7zXZ\x00rest", "xz"},
		{"x.tar.bz2", "BZh91AY", "bzip2"},
		{"x.tar.zst", "\x28\xb5\x2f\xfdrest", "zstd"},
	} {
		path := write(t, filepath.Join(root, c.name), []byte(c.magic))
		_, err := call(t, "open", args(path))
		wantErr(t, err, object.ParseError, `open() "`+path+`" is `+c.what+`-compressed; tar reads plain and gzip-compressed archives`)
	}

	// A .tar.gz whose Writer was never closed fails at the first step.
	unfinished := filepath.Join(root, "u.tar.gz")
	w := mustCall(t, "create", args(unfinished), kw("gzip", object.True))
	mustInvoke(t, w, "add_data", args("a.txt", strings.Repeat("x", 100)))
	r = mustCall(t, "open", args(unfinished))
	_, err = object.Collect(r)
	wantErr(t, err, object.ParseError, `"`+unfinished+`" is not a tar archive`)
	mustInvoke(t, r, "close", nil)
	mustInvoke(t, w, "close", nil)
}

func TestStreams(t *testing.T) {
	data := gzipped(t, tarBytes(t, file("a.txt", "hello"), file("b.txt", "world")))
	closed := false
	r := mustCall(t, "Reader", args(stream(data, &closed)))
	if r.(*Reader).String() != "<tar.Reader>" {
		t.Fatalf("shows as %v", r)
	}
	if p, _ := r.GetAttr("path"); p != object.Nil {
		t.Fatalf("path = %v", p)
	}
	it, _ := object.Iterate(r)
	m, _, _ := it.Next()
	f := mustInvoke(t, r, "open", args(m))
	if s, _ := f.ToString(); s != "<File a.txt>" {
		t.Fatalf("member File shows as %s", s)
	}
	if got := mustInvoke(t, f, "read", nil); string(got.(object.Bytes)) != "hello" {
		t.Fatalf("read = %v", got)
	}
	mustInvoke(t, r, "close", nil)
	if closed {
		t.Fatal("Reader.close() closed the source")
	}

	_, err := call(t, "Reader", args(stream([]byte("\xfd7zXZ\x00"), &closed)))
	wantErr(t, err, object.ParseError, "Reader() the data is xz-compressed; tar reads plain and gzip-compressed archives")

	// Over a stream, a refusal names no archive and carries no path.
	hostile := tarBytes(t, file("../x", "evil"))
	r = mustCall(t, "Reader", args(stream(hostile, &closed)))
	it, _ = object.Iterate(r)
	m, _, _ = it.Next()
	dst := filepath.Join(t.TempDir(), "dst")
	_, err = invoke(t, r, "extract", args(m, dst))
	e := wantErr(t, err, archive.UnsafeError, `extract() member "../x" would be written outside "`+dst+`"`)
	if field(t, e, "path") != object.Nil {
		t.Fatal("path over a stream is not nil")
	}

	var buf bytes.Buffer
	targetClosed := false
	w := mustCall(t, "Writer", args(sinkValue(&buf, &targetClosed)))
	if w.(*Writer).String() != "<tar.Writer>" {
		t.Fatalf("shows as %v", w)
	}
	mustInvoke(t, w, "add_data", args("a.txt", "hi"))
	mustInvoke(t, w, "close", nil)
	if targetClosed {
		t.Fatal("Writer.close() closed the target")
	}
	tr := gotar.NewReader(&buf)
	h, err := tr.Next()
	if err != nil || h.Name != "a.txt" {
		t.Fatalf("written %v, %v", h, err)
	}
	if body, _ := io.ReadAll(tr); string(body) != "hi" {
		t.Fatalf("body = %q", body)
	}

	w = mustCall(t, "Writer", args(sinkValue(&buf, &targetClosed)))
	_, err = invoke(t, w, "add_data", args("a.txt", stream(nil, &closed)))
	wantErr(t, err, object.TypeError, "add_data() argument 'data' must be String or Bytes, got Module (a tar header holds the size before the content; write a stream to a file and add() it)")
}

func TestWriterErrors(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	os.Mkdir(src, 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644)
	own := filepath.Join(src, "self.tar")
	w := mustCall(t, "create", args(own))
	mustInvoke(t, w, "add", args(src))
	_, err := invoke(t, w, "add", args(filepath.Join(src, "a.txt")), kw("name", object.String("src/a.txt")))
	wantErr(t, err, object.ValueError, `add() "src/a.txt" is already in the archive`)
	_, err = invoke(t, w, "add", args("."))
	wantErr(t, err, object.ValueError, `add() "." has no name; pass name=`)
	_, err = invoke(t, w, "add_data", args("a/../../b", "x"))
	wantErr(t, err, object.ValueError, `add_data() name "a/../../b" would be unpacked outside its directory`)
	mustInvoke(t, w, "close", nil)
	r := mustCall(t, "open", args(own))
	if got := names(t, r); !reflect.DeepEqual(got, []string{"src", "src/a.txt"}) {
		t.Fatalf("members = %v", got)
	}
	mustInvoke(t, r, "close", nil)
	_, err = object.Collect(r)
	wantErr(t, err, object.ValueError, "cannot iterate a closed archive")
}
