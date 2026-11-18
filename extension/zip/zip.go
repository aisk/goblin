// Package zip implements the zip module: reading a zip archive member by
// member, unpacking it under a directory with nothing landing outside it,
// and writing one.
package zip

import (
	gozip "archive/zip"
	"compress/flate"
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aisk/goblin/extension/internal/archive"
	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

var (
	readerType = object.NewOpaqueConstructor("Reader", "zip.open(path)")
	writerType = object.NewOpaqueConstructor("Writer", "zip.create(path)")
)

func Execute() (object.Object, error) {
	return object.NewModule("zip", map[string]object.Object{
		"Reader":      readerType.Function,
		"Writer":      writerType.Function,
		"Member":      archive.MemberType.Function,
		"UnsafeError": archive.UnsafeError,
	}, object.Funcs{
		"open":    open,
		"create":  create,
		"extract": extract,
	}), nil
}

// The host systems whose zip entries carry Unix permission bits.
const (
	creatorUnix  = 3
	creatorMacOS = 19
)

// Reader is an open zip archive. Its index is read once, at open; member
// content is read only when asked for.
type Reader struct {
	object.OpaqueBase
	mu      sync.Mutex
	path    *pathvalue.Path
	file    *os.File
	files   []*gozip.File
	members []*archive.Member
	byName  map[string]int
	// linkErrs holds why a link's target could not be read at open.
	linkErrs map[int]error
	closed   atomic.Bool
}

// openReader opens the zip at path for fn, which names the call in
// messages.
func openReader(fn, path string) (*Reader, error) {
	clean := pathvalue.NewPath(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, filestream.Fail(fn, "cannot open", clean.String(), err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, filestream.Fail(fn, "cannot open", clean.String(), err)
	}
	if info.IsDir() {
		// Reported as gzip and tar report it, as a file that cannot be
		// read rather than as data in the wrong format.
		file.Close()
		return nil, filestream.Fail(fn, "cannot read", clean.String(), syscall.EISDIR)
	}
	zr, err := gozip.NewReader(file, info.Size())
	// Go may report names it considers insecure together with a usable
	// reader; listing must show them, and extract does its own check.
	if err != nil && !errors.Is(err, gozip.ErrInsecurePath) {
		file.Close()
		var pathErr *stdfs.PathError
		if errors.As(err, &pathErr) {
			return nil, filestream.Fail(fn, "cannot read", clean.String(), err)
		}
		return nil, notZip(fn, clean)
	}
	r := &Reader{
		OpaqueBase: object.MakeOpaqueBase("Reader"),
		path:       clean,
		file:       file,
		files:      zr.File,
		byName:     map[string]int{},
		linkErrs:   map[int]error{},
	}
	for i, f := range zr.File {
		m := newMember(r, f)
		m.Pos = i
		if m.Kind == archive.KindSymlink {
			target, err := readLink(f)
			if err != nil {
				r.linkErrs[i] = err
			} else {
				m.Target, m.HasTarget = target, true
			}
		}
		r.members = append(r.members, m)
		r.byName[m.Display] = i
	}
	return r, nil
}

func notZip(fn string, path *pathvalue.Path) error {
	return object.NewParseError("%s() %s is not a zip archive", fn, archive.Quote(path.String())).With("path", path)
}

// readLink reads a link member's content, which is its target.
func readLink(f *gozip.File) (string, error) {
	if f.Flags&0x1 != 0 {
		return "", gozip.ErrAlgorithm
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 4096))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func newMember(owner *Reader, f *gozip.File) *archive.Member {
	mode := f.Mode()
	kind := archive.KindFile
	switch {
	case mode.IsDir():
		kind = archive.KindDir
	case mode&stdfs.ModeSymlink != 0:
		kind = archive.KindSymlink
	case mode&stdfs.ModeDevice != 0:
		kind = archive.KindDevice
	case mode&stdfs.ModeNamedPipe != 0:
		kind = archive.KindFIFO
	case !mode.IsRegular():
		kind = archive.KindOther
	}
	perm := int64(mode.Perm())
	if creator := f.CreatorVersion >> 8; creator != creatorUnix && creator != creatorMacOS {
		perm = 0o644
		if kind == archive.KindDir {
			perm = 0o755
		}
	}
	size := int64(f.UncompressedSize64)
	if size < 0 {
		size = int64(^uint64(0) >> 1)
	}
	// Go gives the time in an unnamed fixed zone (or UTC), which no zone=
	// accepts; like fs.stat and tar, a member's time is in the local zone.
	return archive.NewMember(owner, f.Name, kind, size, f.Modified.Local(), perm)
}

func open(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("open", args)
	path := p.Path("path")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return openReader("open", path)
}

func (r *Reader) Constructor() *object.Function { return readerType.Function }
func (r *Reader) String() string                { return "<zip.Reader " + r.path.String() + ">" }
func (r *Reader) ToString() (string, error)     { return r.String(), nil }

func (r *Reader) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Reader)
	return ok && v == r, nil
}

func (r *Reader) GetAttr(name string) (object.Object, error) {
	switch name {
	case "path":
		return r.path, nil
	case "closed":
		return object.Bool(r.closed.Load()), nil
	case "find":
		return &object.Function{Name: "find", Fn: r.find}, nil
	case "open":
		return &object.Function{Name: "open", Fn: r.openMember}, nil
	case "extract":
		return &object.Function{Name: "extract", Fn: r.extractMember}, nil
	case "close":
		return &object.Function{Name: "close", Fn: r.close}, nil
	}
	return nil, object.NewAttributeError("Reader has no attribute '%s'", name)
}

func (r *Reader) Attributes() []string {
	return []string{"path", "closed", "find", "open", "extract", "close"}
}

func (r *Reader) Iter() ([]object.Object, error) { return object.Collect(r) }

// Iterator gives every Member in the order the archive lists them; each
// pass starts from the first.
func (r *Reader) Iterator() (object.Iterator, error) {
	if r.closed.Load() {
		return nil, object.NewValueError("cannot iterate a closed archive")
	}
	i := 0
	return object.IteratorFunc(func() (object.Object, bool, error) {
		if r.closed.Load() {
			return nil, false, object.NewValueError("cannot iterate a closed archive")
		}
		if i >= len(r.members) {
			return nil, false, nil
		}
		m := r.members[i]
		i++
		return m, true, nil
	}), nil
}

func (r *Reader) find(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("find", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if r.closed.Load() {
		return nil, archive.Closed("find")
	}
	if i, ok := r.byName[string(name)]; ok {
		return r.members[i], nil
	}
	return object.Nil, nil
}

// member resolves a member argument: a Member of this Reader, or a name.
func (r *Reader) member(fn string, v object.Object) (*archive.Member, error) {
	switch m := v.(type) {
	case *archive.Member:
		if m.Owner != r {
			return nil, object.NewValueError("%s() member %s is not from %s", fn, archive.Quote(m.Display), archive.Quote(r.path.String()))
		}
		return m, nil
	case object.String:
		i, ok := r.byName[string(m)]
		if !ok {
			return nil, object.NewKeyError("%s() no member %s in %s", fn, archive.Quote(string(m)), archive.Quote(r.path.String()))
		}
		return r.members[i], nil
	}
	return nil, object.NewTypeError("%s() argument 'member' must be Member or String, got %s", fn, v.TypeName())
}

// openContent opens member m's content for fn: encrypted members and
// compression methods Go does not read are ParseErrors.
func (r *Reader) openContent(fn string, m *archive.Member) (io.ReadCloser, error) {
	f := r.files[m.Pos]
	if f.Flags&0x1 != 0 {
		return nil, object.NewParseError("%s() member %s of %s is encrypted", fn, archive.Quote(m.Display), archive.Quote(r.path.String())).With("path", r.path)
	}
	rc, err := f.Open()
	if err != nil {
		if errors.Is(err, gozip.ErrAlgorithm) {
			return nil, object.NewParseError("%s() member %s of %s uses a compression method zip does not read", fn, archive.Quote(m.Display), archive.Quote(r.path.String())).With("path", r.path)
		}
		if corrupt(err) {
			return nil, archive.CorruptMember(fn, m, r.path.String(), r.path)
		}
		return nil, filestream.Fail(fn, "cannot read", r.path.String(), err)
	}
	return rc, nil
}

// corrupt reports whether err from a member's content is damage in the
// archive.
func corrupt(err error) bool {
	var flateErr flate.CorruptInputError
	return errors.Is(err, gozip.ErrChecksum) || errors.Is(err, gozip.ErrFormat) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &flateErr)
}

func (r *Reader) openMember(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("open", args)
	v := p.Any("member")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return nil, archive.Closed("open")
	}
	m, err := r.member("open", v)
	if err != nil {
		return nil, err
	}
	if err := m.NotContent("open"); err != nil {
		return nil, err
	}
	rc, err := r.openContent("open", m)
	if err != nil {
		return nil, err
	}
	return archive.MemberFile(&archive.Content{
		R:           rc,
		Alive:       func() bool { return !r.closed.Load() },
		Member:      m,
		Archive:     r.path.String(),
		ArchivePath: r.path,
		Corrupt:     corrupt,
	}), nil
}

// extractor prepares an Extractor over dst for this archive.
func (r *Reader) extractor(dst string, overwrite bool) *archive.Extractor {
	x := archive.NewExtractor(dst)
	x.Archive = r.path.String()
	x.ArchivePath = r.path
	x.Overwrite = overwrite
	x.ReadError = func(m *archive.Member, err error) error {
		if errors.Is(err, gozip.ErrFormat) {
			// Go's reader raises ErrFormat when a member gives more
			// bytes than its header declares.
			return archive.ErrOversize
		}
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return err
		}
		if corrupt(err) {
			return archive.CorruptMember("extract", m, r.path.String(), r.path)
		}
		return filestream.Fail("extract", "cannot read", r.path.String(), err)
	}
	return x
}

// extractOne writes m under x at name.
func (r *Reader) extractOne(x *archive.Extractor, m *archive.Member, name string) (string, error) {
	if m.Kind == archive.KindSymlink && !m.HasTarget && archive.SafeName(name) {
		err := r.linkErrs[m.Pos]
		if errors.Is(err, gozip.ErrAlgorithm) {
			_, err = r.openContent("extract", m)
			return "", err
		}
		if corrupt(err) {
			return "", archive.CorruptMember("extract", m, r.path.String(), r.path)
		}
		return "", filestream.Fail("extract", "cannot read", r.path.String(), err)
	}
	return x.Extract(m, name, func() (io.ReadCloser, error) {
		return r.openContent("extract", m)
	})
}

func (r *Reader) extractMember(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("extract", args)
	v := p.Any("member")
	dst := p.Path("dst")
	name, hasName := p.NullablePath("name")
	overwrite := bool(p.BoolOr("overwrite", false))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return nil, archive.Closed("extract")
	}
	m, err := r.member("extract", v)
	if err != nil {
		return nil, err
	}
	landing := m.Raw
	if hasName {
		landing = archive.LandingName(name)
	}
	x := r.extractor(dst, overwrite)
	defer x.Close()
	written, err := r.extractOne(x, m, landing)
	if err != nil {
		return nil, err
	}
	return pathvalue.NewPath(written), nil
}

func (r *Reader) close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("close", args); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Swap(true) {
		return object.Nil, nil
	}
	if err := r.file.Close(); err != nil {
		return nil, filestream.Fail("close", "cannot close", r.path.String(), err)
	}
	return object.Nil, nil
}

func maxSizeArg(p *object.ArgParser) (int64, bool) {
	v, ok := p.NullableInt("max_size")
	return int64(v), ok
}

// extract is the one-call unpacking of a whole archive. A zip's index is
// available up front, so every member's name, kind and declared size is
// checked before anything is written.
func extract(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("extract", args)
	path := p.Path("path")
	dst := p.Path("dst")
	overwrite := bool(p.BoolOr("overwrite", false))
	maxSize, limited := maxSizeArg(p)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if limited && maxSize < 0 {
		return nil, object.NegativeArgumentError("extract", "max_size", int64(maxSize))
	}
	r, err := openReader("extract", path)
	if err != nil {
		return nil, err
	}
	defer r.file.Close()
	label := r.path.String()
	total := int64(0)
	for _, m := range r.members {
		if err := archive.NameRefusal(m, label, r.path, dst); err != nil {
			return nil, err
		}
		if err := archive.KindRefusal(m, label, r.path); err != nil {
			return nil, err
		}
		total = addSize(total, m.Size)
	}
	if limited && total > maxSize {
		return nil, archive.Unsafe(nil, r.path, "extract() %s unpacks to %d bytes, over max_size %d", archive.Quote(label), total, maxSize)
	}
	x := r.extractor(dst, overwrite)
	defer x.Close()
	if err := x.Open(); err != nil {
		return nil, err
	}
	for _, m := range r.members {
		if _, err := r.extractOne(x, m, m.Raw); err != nil {
			return nil, err
		}
	}
	return object.Nil, nil
}

// addSize adds without overflowing: a sum past the largest Int stays there.
func addSize(total, size int64) int64 {
	if total > (1<<63-1)-size {
		return 1<<63 - 1
	}
	return total + size
}

// Writer is a zip being written. The file is not a valid archive until
// close() writes the index.
type Writer struct {
	object.OpaqueBase
	mu     sync.Mutex
	path   *pathvalue.Path
	file   *os.File
	zw     *gozip.Writer
	adder  archive.Adder
	closed bool
}

func create(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("create", args)
	path := p.Path("path")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	clean := pathvalue.NewPath(path)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, filestream.Fail("create", "cannot open", clean.String(), err)
	}
	w := &Writer{OpaqueBase: object.MakeOpaqueBase("Writer"), path: clean, file: file, zw: gozip.NewWriter(file)}
	w.adder.Sink = sink{w.zw}
	if info, err := file.Stat(); err == nil {
		w.adder.Own = info
	}
	w.adder.WriteError = func(fn string, err error) error {
		return filestream.Fail(fn, "cannot write", clean.String(), err)
	}
	return w, nil
}

// sink stores entries in Go's zip writer.
type sink struct{ zw *gozip.Writer }

func (s sink) header(name string, info stdfs.FileInfo, method uint16) (*gozip.FileHeader, error) {
	h, err := gozip.FileInfoHeader(info)
	if err != nil {
		return nil, err
	}
	h.Name = name
	h.Method = method
	return h, nil
}

func (s sink) WriteDir(name string, info stdfs.FileInfo) error {
	h, err := s.header(name+"/", info, gozip.Store)
	if err != nil {
		return err
	}
	_, err = s.zw.CreateHeader(h)
	return err
}

func (s sink) WriteFile(name string, info stdfs.FileInfo, content io.Reader) error {
	h, err := s.header(name, info, gozip.Deflate)
	if err != nil {
		return err
	}
	w, err := s.zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, content)
	return err
}

func (s sink) WriteLink(name string, info stdfs.FileInfo, target string) error {
	h, err := s.header(name, info, gozip.Store)
	if err != nil {
		return err
	}
	w, err := s.zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, target)
	return err
}

func (w *Writer) Constructor() *object.Function { return writerType.Function }
func (w *Writer) String() string                { return "<zip.Writer " + w.path.String() + ">" }
func (w *Writer) ToString() (string, error)     { return w.String(), nil }

func (w *Writer) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Writer)
	return ok && v == w, nil
}

func (w *Writer) GetAttr(name string) (object.Object, error) {
	switch name {
	case "path":
		return w.path, nil
	case "closed":
		w.mu.Lock()
		defer w.mu.Unlock()
		return object.Bool(w.closed), nil
	case "add":
		return &object.Function{Name: "add", Fn: w.add}, nil
	case "add_data":
		return &object.Function{Name: "add_data", Fn: w.addData}, nil
	case "close":
		return &object.Function{Name: "close", Fn: w.close}, nil
	}
	return nil, object.NewAttributeError("Writer has no attribute '%s'", name)
}

func (w *Writer) Attributes() []string {
	return []string{"path", "closed", "add", "add_data", "close"}
}

func (w *Writer) add(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("add", args)
	path := p.Path("path")
	name, hasName := p.NullablePath("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, archive.Closed("add")
	}
	if err := w.adder.Add("add", path, name, hasName); err != nil {
		return nil, err
	}
	return object.Nil, nil
}

func (w *Writer) addData(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("add_data", args)
	given := p.Path("name")
	data := p.Data("data")
	mode := p.IntOr("mode", 0o644)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, archive.Closed("add_data")
	}
	name, err := archive.WriterName("add_data", given)
	if err != nil {
		return nil, err
	}
	if err := archive.ModeArg("add_data", mode); err != nil {
		return nil, err
	}
	if err := w.adder.Claim("add_data", name); err != nil {
		return nil, err
	}
	h := &gozip.FileHeader{Name: name, Method: gozip.Deflate, Modified: time.Now()}
	h.SetMode(stdfs.FileMode(mode))
	out, err := w.zw.CreateHeader(h)
	if err != nil {
		return nil, w.adder.WriteError("add_data", err)
	}
	if _, err := io.Copy(out, data); err != nil {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return nil, err
		}
		return nil, w.adder.WriteError("add_data", err)
	}
	return object.Nil, nil
}

// close writes the index, then releases the file; the first error wins.
func (w *Writer) close(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("close", args); err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return object.Nil, nil
	}
	w.closed = true
	err := w.zw.Close()
	if closeErr := w.file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, filestream.Fail("close", "cannot write", w.path.String(), err)
	}
	return object.Nil, nil
}
