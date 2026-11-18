package archive

import (
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

// ErrOversize is what an Extractor's ReadError returns when the member's
// content turned out longer than its header declared.
var ErrOversize = errors.New("member larger than declared")

// Extractor writes members under one directory. Every write goes through an
// *os.Root opened on that directory, which refuses any path that would
// leave it, including through a symbolic link, without a check-then-use
// race. The lexical checks before it give the refusals a clear message.
type Extractor struct {
	root *os.Root
	// dst is the directory as the caller named it, for messages and for
	// the paths extract returns.
	dst   string
	given string
	// Archive names the archive in messages ("" over a stream);
	// ArchivePath is the Path errors carry, nil over a stream.
	Archive     string
	ArchivePath object.Object
	Overwrite   bool
	// ReadError maps an error reading a member's content: ErrOversize when
	// the member is longer than it declares, otherwise the Goblin error to
	// raise.
	ReadError func(m *Member, err error) error
}

// NewExtractor prepares to write members under dst. Nothing is touched
// until Open, or the first Extract.
func NewExtractor(dst string) *Extractor {
	return &Extractor{dst: filepath.Clean(dst), given: dst, ArchivePath: object.Nil}
}

// Open creates the directory with its parents when missing and opens it.
// Calling it again does nothing.
func (x *Extractor) Open() error {
	if x.root != nil {
		return nil
	}
	if err := os.MkdirAll(x.given, 0o755); err != nil {
		if errors.Is(err, syscall.ENOTDIR) || errors.Is(err, stdfs.ErrExist) {
			return ExistErr(x.dst, "is not a directory")
		}
		return filestream.Fail("extract", "cannot create", x.dst, err)
	}
	root, err := os.OpenRoot(x.given)
	if err != nil {
		return filestream.Fail("extract", "cannot open", x.dst, err)
	}
	x.root = root
	return nil
}

// Close releases the directory.
func (x *Extractor) Close() {
	if x.root != nil {
		x.root.Close()
		x.root = nil
	}
}

// ExistErr is the ExistError about a path under the destination.
func ExistErr(p, what string) error {
	return object.NewKindError(object.ExistError, "extract() %s %s", Quote(p), what).
		With("path", pathvalue.NewPath(p))
}

func (x *Extractor) of() string {
	if x.Archive == "" {
		return ""
	}
	return " of " + Quote(x.Archive)
}

// Unsafe builds the UnsafeError for member m (nil when the refusal is
// about the whole archive).
func Unsafe(m *Member, archivePath object.Object, format string, a ...any) error {
	var member object.Object = object.Nil
	if m != nil {
		member = m
	}
	if archivePath == nil {
		archivePath = object.Nil
	}
	return object.NewKindError(UnsafeError, format, a...).With("member", member).With("path", archivePath)
}

func (x *Extractor) unsafe(m *Member, format string, a ...any) error {
	return Unsafe(m, x.ArchivePath, format, a...)
}

// outside is the refusal of a name that would land outside the directory.
func (x *Extractor) outside(m *Member, name string) error {
	as := ""
	if name != m.Raw {
		as = " as " + Quote(strings.ToValidUTF8(name, "�"))
	}
	return x.unsafe(m, "extract() member %s%s would be written outside %s%s", Quote(m.Display), x.of(), Quote(x.dst), as)
}

// join is where name lands on disk.
func (x *Extractor) join(name string) string {
	return filepath.Join(x.dst, filepath.FromSlash(name))
}

// CheckKind refuses a member that is no file, directory or link.
func (x *Extractor) CheckKind(m *Member) error {
	return KindRefusal(m, x.Archive, x.ArchivePath)
}

// KindRefusal is the UnsafeError for a device, FIFO or other special
// member, or nil for a member extract can write. archive names the archive
// in the message ("" over a stream).
func KindRefusal(m *Member, archive string, archivePath object.Object) error {
	of := ""
	if archive != "" {
		of = " of " + Quote(archive)
	}
	switch m.Kind {
	case KindDevice:
		return Unsafe(m, archivePath, "extract() member %s%s is a device", Quote(m.Display), of)
	case KindFIFO:
		return Unsafe(m, archivePath, "extract() member %s%s is a FIFO", Quote(m.Display), of)
	case KindOther:
		return Unsafe(m, archivePath, "extract() member %s%s is not a file, directory or link", Quote(m.Display), of)
	}
	return nil
}

// NameRefusal is the UnsafeError for a member whose own name would land
// outside dst, or nil.
func NameRefusal(m *Member, archive string, archivePath object.Object, dst string) error {
	if SafeName(m.Raw) {
		return nil
	}
	of := ""
	if archive != "" {
		of = " of " + Quote(archive)
	}
	return Unsafe(m, archivePath, "extract() member %s%s would be written outside %s", Quote(m.Display), of, Quote(filepath.Clean(dst)))
}

// Extract writes member m under the directory at name (its own name unless
// the caller gave another) and returns the path written. content opens the
// member's content; it is called only for a file, and what it returns is
// closed afterwards.
func (x *Extractor) Extract(m *Member, name string, content func() (io.ReadCloser, error)) (string, error) {
	if !SafeName(name) {
		return "", x.outside(m, name)
	}
	if err := x.CheckKind(m); err != nil {
		return "", err
	}
	if err := x.Open(); err != nil {
		return "", err
	}
	local := filepath.FromSlash(name)
	switch m.Kind {
	case KindDir:
		return x.join(name), x.dir(m, name, local)
	case KindSymlink:
		if !LinkTargetSafe(name, m.Target) {
			return "", x.unsafe(m, "extract() link %s%s points outside %s: %s", Quote(m.Display), x.of(), Quote(x.dst), Quote(m.Target))
		}
		if err := x.prepare(m, name, local); err != nil {
			return "", err
		}
		if err := x.root.Symlink(filepath.FromSlash(m.Target), local); err != nil {
			return "", x.classify(m, name, name, false, err)
		}
		return x.join(name), nil
	case KindHardLink:
		target := strings.TrimSuffix(m.Target, "/")
		if !SafeName(target) {
			return "", x.unsafe(m, "extract() link %s%s points outside %s: %s", Quote(m.Display), x.of(), Quote(x.dst), Quote(m.Target))
		}
		if err := x.prepare(m, name, local); err != nil {
			return "", err
		}
		if err := x.root.Link(filepath.FromSlash(target), local); err != nil {
			return "", x.classify(m, name, name, false, err)
		}
		return x.join(name), nil
	}
	return x.join(name), x.file(m, name, local, content)
}

// parents creates the directories above name that are missing; many
// archives have no entries for them.
func (x *Extractor) parents(m *Member, name string) error {
	dir := path.Dir(name)
	if dir == "." || dir == "/" {
		return nil
	}
	if err := x.root.MkdirAll(filepath.FromSlash(dir), 0o755); err != nil {
		return x.classify(m, name, dir, true, err)
	}
	return nil
}

// prepare makes room for a file or link at name: an existing directory is
// in the way whatever overwrite says, any other entry only without it. With
// overwrite the entry is removed, so nothing is written through an
// existing link.
func (x *Extractor) prepare(m *Member, name, local string) error {
	if err := x.parents(m, name); err != nil {
		return err
	}
	info, err := x.root.Lstat(local)
	if err != nil {
		if errors.Is(err, stdfs.ErrNotExist) {
			return nil
		}
		return x.classify(m, name, name, false, err)
	}
	if info.IsDir() {
		return ExistErr(x.join(name), "is a directory")
	}
	if !x.Overwrite {
		return ExistErr(x.join(name), "already exists (pass overwrite=true)")
	}
	if err := x.root.Remove(local); err != nil {
		return x.classify(m, name, name, false, err)
	}
	return nil
}

func (x *Extractor) file(m *Member, name, local string, content func() (io.ReadCloser, error)) error {
	if err := x.prepare(m, name, local); err != nil {
		return err
	}
	r, err := content()
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := x.root.OpenFile(local, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(m.Mode&0o777))
	if err != nil {
		return x.classify(m, name, name, false, err)
	}
	written, err := io.CopyN(out, readSide{r}, m.Size+1)
	closeErr := out.Close()
	if err == io.EOF {
		err = nil
	}
	if err == nil && written > m.Size {
		err = ErrOversize
	}
	if err != nil {
		var readErr readFailure
		if errors.As(err, &readErr) {
			err = readErr.err
			if x.ReadError != nil {
				err = x.ReadError(m, err)
			}
		} else if !errors.Is(err, ErrOversize) {
			x.root.Remove(local)
			return filestream.Fail("extract", "cannot write", x.join(name), err)
		}
		x.root.Remove(local)
		if errors.Is(err, ErrOversize) {
			return x.unsafe(m, "extract() member %s%s is larger than the %d bytes it declares", Quote(m.Display), x.of(), m.Size)
		}
		return err
	}
	if closeErr != nil {
		return filestream.Fail("extract", "cannot write", x.join(name), closeErr)
	}
	if !m.ModTime.IsZero() {
		if err := x.root.Chtimes(local, m.ModTime, m.ModTime); err != nil {
			return filestream.Fail("extract", "cannot set the time of", x.join(name), err)
		}
	}
	return nil
}

// readSide marks errors that come from reading the member, so they can be
// told from errors writing the file.
type readSide struct{ r io.Reader }

type readFailure struct{ err error }

func (e readFailure) Error() string { return e.err.Error() }

func (r readSide) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && err != io.EOF {
		return n, readFailure{err}
	}
	return n, err
}

func (x *Extractor) dir(m *Member, name, local string) error {
	if err := x.parents(m, name); err != nil {
		return err
	}
	perm := os.FileMode(m.Mode&0o777) | 0o700
	info, err := x.root.Lstat(local)
	switch {
	case err != nil && errors.Is(err, stdfs.ErrNotExist):
		if err := x.root.Mkdir(local, perm); err != nil {
			return x.classify(m, name, name, false, err)
		}
		return nil
	case err != nil:
		return x.classify(m, name, name, false, err)
	case info.IsDir():
		return nil
	case info.Mode()&stdfs.ModeSymlink != 0:
		if target, err := x.root.Stat(local); err == nil && target.IsDir() {
			return nil
		}
		if !x.Overwrite {
			return ExistErr(x.join(name), "already exists (pass overwrite=true)")
		}
		if err := x.root.Remove(local); err != nil {
			return x.classify(m, name, name, false, err)
		}
		if err := x.root.Mkdir(local, perm); err != nil {
			return x.classify(m, name, name, false, err)
		}
		return nil
	}
	return ExistErr(x.join(name), "is not a directory")
}

// classify reports a failed operation on at, a path on the way to name
// (the name the member lands at) or name itself. os.Root refuses a path that
// leaves the directory, through ".." or a symbolic link, but does not export
// the error it returns for that, so the failure is recognised by its text or
// by a link among the components of at: that is an UnsafeError. A file where
// a directory must go is an ExistError; anything else keeps its fs kind.
// dirWanted says at itself must be a directory.
func (x *Extractor) classify(m *Member, name, at string, dirWanted bool, err error) error {
	if strings.Contains(err.Error(), "path escapes from parent") {
		return x.outside(m, name)
	}
	parts := strings.Split(at, "/")
	sawLink := false
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		info, lerr := x.root.Lstat(filepath.FromSlash(prefix))
		if lerr != nil {
			if strings.Contains(lerr.Error(), "path escapes from parent") {
				return x.outside(m, name)
			}
			break
		}
		if info.Mode()&stdfs.ModeSymlink != 0 {
			sawLink = true
			continue
		}
		if !info.IsDir() && (i < len(parts)-1 || dirWanted) {
			return ExistErr(x.join(prefix), "is not a directory")
		}
	}
	if sawLink && !errors.Is(err, stdfs.ErrPermission) && !errors.Is(err, stdfs.ErrNotExist) {
		return x.outside(m, name)
	}
	return filestream.Fail("extract", "cannot write", x.join(at), err)
}
