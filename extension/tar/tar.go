// Package tar implements the tar module: reading a tar archive, plain or
// gzip-compressed, one member at a time from a file or any readable;
// unpacking it under a directory with nothing landing outside it; and
// writing one to a file or any writable.
package tar

import (
	gotar "archive/tar"
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aisk/goblin/extension/internal/archive"
	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

var (
	readerType = object.NewNativeConstructor("Reader", newReader)
	writerType = object.NewNativeConstructor("Writer", newWriter)
)

func Execute() (object.Object, error) {
	return object.NewModule("tar", map[string]object.Object{
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

// Reader is a tar being read: a stream of members, visited in order.
type Reader struct {
	object.OpaqueBase
	mu sync.Mutex
	// path is the archive's Path, nil over a stream; file is what open
	// opened, nil over a stream, which the caller owns.
	path *pathvalue.Path
	file *os.File
	tr   *gotar.Reader
	// current is the Member the latest step gave; gen counts steps, so a
	// member File knows when the loop has moved past its member.
	current *archive.Member
	gen     atomic.Uint64
	steps   int
	// consumed counts the bytes of the current member read through its
	// Files.
	consumed int64
	closed   atomic.Bool
}

// label names the archive in messages, "" over a stream.
func (r *Reader) label() string {
	if r.path == nil {
		return ""
	}
	return r.path.String()
}

// pathData is the archive's Path for error data, nil over a stream.
func (r *Reader) pathData() object.Object {
	if r.path == nil {
		return object.Nil
	}
	return r.path
}

// prefix is what a message starts with for fn ("" for a loop).
func prefix(fn string) string {
	if fn == "" {
		return ""
	}
	return fn + "() "
}

// decompressed recognises the compression of src by its first bytes:
// gzip is read through, and the formats tar does not read are named.
func decompressed(fn string, src io.Reader, path *pathvalue.Path) (io.Reader, error) {
	br := bufio.NewReader(src)
	magic, err := br.Peek(6)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return nil, err
		}
		if path != nil {
			return nil, filestream.Fail(fn, "cannot read", path.String(), err)
		}
		return nil, object.WrapNativeError(object.IOError, fn+"() cannot read the archive", err)
	}
	compression := ""
	switch {
	case bytes.HasPrefix(magic, []byte{0x1f, 0x8b}):
		zr, err := gzip.NewReader(br)
		if err != nil {
			var goblinErr *object.Error
			if errors.As(err, &goblinErr) {
				return nil, err
			}
			return nil, parseErr(fn, path, "corrupt")
		}
		return zr, nil
	case bytes.HasPrefix(magic, []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}):
		compression = "xz"
	case bytes.HasPrefix(magic, []byte("BZh")):
		compression = "bzip2"
	case bytes.HasPrefix(magic, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		compression = "zstd"
	}
	if compression != "" {
		what := "the data"
		var data object.Object = object.Nil
		if path != nil {
			what = archive.Quote(path.String())
			data = path
		}
		return nil, object.NewParseError("%s%s is %s-compressed; tar reads plain and gzip-compressed archives", prefix(fn), what, compression).With("path", data)
	}
	return br, nil
}

// parseErr is the ParseError for data that is not a tar ("not") or is
// damaged ("corrupt").
func parseErr(fn string, path *pathvalue.Path, which string) error {
	var err *object.Error
	switch {
	case which == "not" && path != nil:
		err = object.NewParseError("%s%s is not a tar archive", prefix(fn), archive.Quote(path.String()))
	case which == "not":
		err = object.NewParseError("%snot a tar archive", prefix(fn))
	case path != nil:
		err = object.NewParseError("%scorrupt tar data in %s", prefix(fn), archive.Quote(path.String()))
	default:
		err = object.NewParseError("%scorrupt tar data", prefix(fn))
	}
	if path == nil {
		return err.With("path", object.Nil)
	}
	return err.With("path", path)
}

// corrupt reports whether err from the tar data is damage in it.
func corrupt(err error) bool {
	var flateErr flate.CorruptInputError
	return errors.Is(err, gotar.ErrHeader) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, gzip.ErrChecksum) || errors.Is(err, gzip.ErrHeader) || errors.As(err, &flateErr)
}

func openReader(fn, path string) (*Reader, error) {
	clean := pathvalue.NewPath(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, filestream.Fail(fn, "cannot open", clean.String(), err)
	}
	src, err := decompressed(fn, file, clean)
	if err != nil {
		file.Close()
		return nil, err
	}
	return &Reader{OpaqueBase: object.MakeOpaqueBase("Reader"), path: clean, file: file, tr: gotar.NewReader(src)}, nil
}

func open(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("open", args)
	path := p.Path("path")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return openReader("open", path)
}

// newReader is tar.Reader(source): a Reader over a readable the caller
// owns and closes.
func newReader(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Reader", args)
	source := p.Any("source")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	duck, err := object.NewDuckReader("Reader", "source", source)
	if err != nil {
		return nil, err
	}
	src, err := decompressed("Reader", readOnly{duck}, nil)
	if err != nil {
		return nil, err
	}
	return &Reader{OpaqueBase: object.MakeOpaqueBase("Reader"), tr: gotar.NewReader(src)}, nil
}

// readOnly hides the Close of a stream the caller owns.
type readOnly struct{ r io.Reader }

func (r readOnly) Read(p []byte) (int, error) { return r.r.Read(p) }

func (r *Reader) Constructor() *object.Function { return readerType.Function }

func (r *Reader) String() string {
	if r.path == nil {
		return "<tar.Reader>"
	}
	return "<tar.Reader " + r.path.String() + ">"
}

func (r *Reader) ToString() (string, error) { return r.String(), nil }

func (r *Reader) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Reader)
	return ok && v == r, nil
}

func (r *Reader) GetAttr(name string) (object.Object, error) {
	switch name {
	case "path":
		return r.pathData(), nil
	case "closed":
		return object.Bool(r.closed.Load()), nil
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
	return []string{"path", "closed", "open", "extract", "close"}
}

func kindOf(h *gotar.Header) archive.Kind {
	switch h.Typeflag {
	case gotar.TypeReg, gotar.TypeRegA, gotar.TypeGNUSparse:
		return archive.KindFile
	case gotar.TypeDir:
		return archive.KindDir
	case gotar.TypeSymlink:
		return archive.KindSymlink
	case gotar.TypeLink:
		return archive.KindHardLink
	case gotar.TypeChar, gotar.TypeBlock:
		return archive.KindDevice
	case gotar.TypeFifo:
		return archive.KindFIFO
	}
	return archive.KindOther
}

// step moves to the next member, passing over what is left of the current
// one. fn names the call in messages ("" for a loop). It returns false at
// the end of the archive.
func (r *Reader) step(fn string) (*archive.Member, bool, error) {
	r.gen.Add(1)
	r.current = nil
	r.consumed = 0
	h, err := r.tr.Next()
	if err == io.EOF {
		return nil, false, nil
	}
	if err != nil && !errors.Is(err, gotar.ErrInsecurePath) {
		first := r.steps == 0
		var goblinErr *object.Error
		switch {
		case errors.As(err, &goblinErr):
			return nil, false, err
		case first && (corrupt(err) || errors.Is(err, io.EOF)):
			return nil, false, parseErr(fn, r.path, "not")
		case corrupt(err):
			return nil, false, parseErr(fn, r.path, "corrupt")
		case r.path != nil:
			return nil, false, filestream.Fail(fn, "cannot read", r.path.String(), err)
		}
		return nil, false, object.WrapNativeError(object.IOError, prefix(fn)+"cannot read the archive", err)
	}
	r.steps++
	kind := kindOf(h)
	m := archive.NewMember(r, h.Name, kind, h.Size, h.ModTime, h.Mode&0o777)
	m.Pos = r.steps - 1
	if kind == archive.KindSymlink || kind == archive.KindHardLink {
		m.Target, m.HasTarget = h.Linkname, true
	}
	r.current = m
	return m, true, nil
}

func (r *Reader) Iter() ([]object.Object, error) { return object.Collect(r) }

// Iterator gives the remaining members. The loop does not close the
// Reader, and a loop left early leaves it at that member.
func (r *Reader) Iterator() (object.Iterator, error) {
	if r.closed.Load() {
		return nil, object.NewValueError("cannot iterate a closed archive")
	}
	return object.IteratorFunc(func() (object.Object, bool, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.closed.Load() {
			return nil, false, object.NewValueError("cannot iterate a closed archive")
		}
		m, ok, err := r.step("")
		if err != nil || !ok {
			return nil, false, err
		}
		return m, true, nil
	}), nil
}

// currentMember checks that v is the Member the latest step gave.
func (r *Reader) currentMember(fn string, v object.Object) (*archive.Member, error) {
	m, ok := v.(*archive.Member)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'member' must be Member, got %s", fn, v.TypeName())
	}
	if r.closed.Load() {
		return nil, archive.Closed(fn)
	}
	if m != r.current {
		return nil, object.NewValueError("%s() %s is not the current member; a tar archive is read in order", fn, archive.Quote(m.Display))
	}
	return m, nil
}

// counter counts the bytes of the current member handed out.
type counter struct{ r *Reader }

func (c counter) Read(p []byte) (int, error) {
	n, err := c.r.tr.Read(p)
	c.r.consumed += int64(n)
	return n, err
}

func (r *Reader) openMember(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("open", args)
	v := p.Any("member")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.currentMember("open", v)
	if err != nil {
		return nil, err
	}
	if err := m.NotContent("open"); err != nil {
		return nil, err
	}
	gen := r.gen.Load()
	return archive.MemberFile(&archive.Content{
		R:           counter{r},
		Alive:       func() bool { return !r.closed.Load() && r.gen.Load() == gen },
		Member:      m,
		Archive:     r.label(),
		ArchivePath: r.pathData(),
		Corrupt:     corrupt,
	}), nil
}

// extractor prepares an Extractor over dst for this archive.
func (r *Reader) extractor(dst string, overwrite bool) *archive.Extractor {
	x := archive.NewExtractor(dst)
	x.Archive = r.label()
	x.ArchivePath = r.pathData()
	x.Overwrite = overwrite
	x.ReadError = func(m *archive.Member, err error) error {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return err
		}
		if corrupt(err) {
			return archive.CorruptMember("extract", m, r.label(), r.pathData())
		}
		if r.path != nil {
			return filestream.Fail("extract", "cannot read", r.path.String(), err)
		}
		return object.WrapNativeError(object.IOError, "extract() cannot read the archive", err)
	}
	return x
}

func (r *Reader) content() (io.ReadCloser, error) {
	return io.NopCloser(counter{r}), nil
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
	m, err := r.currentMember("extract", v)
	if err != nil {
		return nil, err
	}
	if m.Kind == archive.KindFile && r.consumed > 0 {
		return nil, object.NewValueError("extract() member %s was already read; a tar archive is read in order", archive.Quote(m.Display))
	}
	landing := m.Raw
	if hasName {
		landing = archive.LandingName(name)
	}
	x := r.extractor(dst, overwrite)
	defer x.Close()
	written, err := x.Extract(m, landing, r.content)
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
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			return nil, filestream.Fail("close", "cannot close", r.path.String(), err)
		}
	}
	return object.Nil, nil
}

// extract unpacks a whole tar, gzip-compressed or not. A tar is a sequence,
// so it stops at the member it refuses; the members before it stay.
func extract(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("extract", args)
	path := p.Path("path")
	dst := p.Path("dst")
	overwrite := bool(p.BoolOr("overwrite", false))
	maxSize, limited := p.NullableInt("max_size")
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
	x := r.extractor(dst, overwrite)
	defer x.Close()
	if err := x.Open(); err != nil {
		return nil, err
	}
	total := int64(0)
	for {
		m, ok, err := r.step("extract")
		if err != nil {
			return nil, err
		}
		if !ok {
			return object.Nil, nil
		}
		if total > (1<<63-1)-m.Size {
			total = 1<<63 - 1
		} else {
			total += m.Size
		}
		if limited && total > int64(maxSize) {
			return nil, archive.Unsafe(m, r.path, "extract() %s unpacks to at least %d bytes, over max_size %d", archive.Quote(r.label()), total, int64(maxSize))
		}
		if _, err := x.Extract(m, m.Raw, r.content); err != nil {
			return nil, err
		}
	}
}

// Writer is a tar being written, to a file or a writable the caller owns.
type Writer struct {
	object.OpaqueBase
	mu     sync.Mutex
	path   *pathvalue.Path
	file   *os.File
	gz     *gzip.Writer
	tw     *gotar.Writer
	adder  archive.Adder
	closed bool
}

func (w *Writer) init(out io.Writer) {
	w.tw = gotar.NewWriter(out)
	w.adder.Sink = sink{w.tw}
	w.adder.WriteError = w.writeError
}

// writeError reports a failure writing the archive: an error the target
// raised passes through.
func (w *Writer) writeError(fn string, err error) error {
	if w.path != nil {
		return filestream.Fail(fn, "cannot write", w.path.String(), err)
	}
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) {
		return err
	}
	return object.WrapNativeError(object.IOError, fn+"() cannot write the archive", err)
}

func create(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("create", args)
	path := p.Path("path")
	compress := bool(p.BoolOr("gzip", false))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	clean := pathvalue.NewPath(path)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, filestream.Fail("create", "cannot open", clean.String(), err)
	}
	w := &Writer{OpaqueBase: object.MakeOpaqueBase("Writer"), path: clean, file: file}
	if info, err := file.Stat(); err == nil {
		w.adder.Own = info
	}
	if compress {
		w.gz = gzip.NewWriter(file)
		w.init(w.gz)
	} else {
		w.init(file)
	}
	return w, nil
}

// newWriter is tar.Writer(target): a Writer over a writable the caller
// owns and closes.
func newWriter(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Writer", args)
	target := p.Any("target")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	duck, err := object.NewDuckWriter("Writer", "target", target)
	if err != nil {
		return nil, err
	}
	w := &Writer{OpaqueBase: object.MakeOpaqueBase("Writer")}
	w.init(writeOnly{duck})
	return w, nil
}

// writeOnly hides the Close of a stream the caller owns.
type writeOnly struct{ w io.Writer }

func (w writeOnly) Write(p []byte) (int, error) { return w.w.Write(p) }

// sink stores entries in Go's tar writer. Owner and group are not stored,
// so an archive does not record who made it; the format is left to Go,
// which writes ustar and adds a pax header only for a member that needs it.
type sink struct{ tw *gotar.Writer }

func (s sink) header(name string, info stdfs.FileInfo, link string) (*gotar.Header, error) {
	h, err := gotar.FileInfoHeader(info, link)
	if err != nil {
		return nil, err
	}
	h.Name = name
	h.Uid, h.Gid, h.Uname, h.Gname = 0, 0, "", ""
	h.Format = gotar.FormatUnknown
	return h, nil
}

func (s sink) WriteDir(name string, info stdfs.FileInfo) error {
	h, err := s.header(name+"/", info, "")
	if err != nil {
		return err
	}
	return s.tw.WriteHeader(h)
}

func (s sink) WriteFile(name string, info stdfs.FileInfo, content io.Reader) error {
	h, err := s.header(name, info, "")
	if err != nil {
		return err
	}
	if err := s.tw.WriteHeader(h); err != nil {
		return err
	}
	_, err = io.Copy(s.tw, content)
	return err
}

func (s sink) WriteLink(name string, info stdfs.FileInfo, target string) error {
	h, err := s.header(name, info, target)
	if err != nil {
		return err
	}
	return s.tw.WriteHeader(h)
}

func (w *Writer) Constructor() *object.Function { return writerType.Function }

func (w *Writer) String() string {
	if w.path == nil {
		return "<tar.Writer>"
	}
	return "<tar.Writer " + w.path.String() + ">"
}

func (w *Writer) ToString() (string, error) { return w.String(), nil }

func (w *Writer) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Writer)
	return ok && v == w, nil
}

func (w *Writer) GetAttr(name string) (object.Object, error) {
	switch name {
	case "path":
		if w.path == nil {
			return object.Nil, nil
		}
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

// addData stores a String or Bytes. A tar header holds the member's size
// before its content, so a stream of unknown length is not accepted.
func (w *Writer) addData(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("add_data", args)
	given := p.Path("name")
	value := p.Any("data")
	mode := p.IntOr("mode", 0o644)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var data []byte
	switch v := value.(type) {
	case object.String:
		data = []byte(v)
	case object.Bytes:
		data = []byte(v)
	default:
		return nil, object.NewTypeError("add_data() argument 'data' must be String or Bytes, got %s (a tar header holds the size before the content; write a stream to a file and add() it)", value.TypeName())
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
	h := &gotar.Header{Typeflag: gotar.TypeReg, Name: name, Size: int64(len(data)), Mode: int64(mode), ModTime: time.Now()}
	if err := w.tw.WriteHeader(h); err != nil {
		return nil, w.writeError("add_data", err)
	}
	if _, err := w.tw.Write(data); err != nil {
		return nil, w.writeError("add_data", err)
	}
	return object.Nil, nil
}

// close ends the tar data, then the gzip data when create compressed it,
// then releases the file create opened; the first error wins. A target the
// caller gave stays open.
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
	err := w.tw.Close()
	if w.gz != nil {
		if gzErr := w.gz.Close(); err == nil {
			err = gzErr
		}
	}
	if w.file != nil {
		if closeErr := w.file.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return nil, w.writeError("close", err)
	}
	return object.Nil, nil
}
