// Package archive holds what the zip and tar modules share: the Member value
// type, the UnsafeError kind, the check on member names, the extractor that
// writes members under a directory, and the helpers both Writers use. Neither
// module imports the other; both import this package, so zip.Member and
// tar.Member are one type and zip.UnsafeError and tar.UnsafeError one kind.
package archive

import (
	"strings"
	"time"

	"github.com/aisk/goblin/extension/internal/timevalue"
	"github.com/aisk/goblin/object"
)

// MemberType is the one constructor identity of Member, exported by both
// modules. A Member only comes from iterating an archive.
var MemberType = object.NewNativeConstructor("Member", func(object.CallArgs) (object.Object, error) {
	return nil, object.NewTypeError("Member cannot be constructed; iterate an archive from zip.open(path) or tar.open(path)")
})

// UnsafeError is the kind extract raises for a member it refuses. It is
// made once here and exported by both modules.
var UnsafeError = object.NewErrorKind("UnsafeError", object.BaseError)

// Kind is what a member is.
type Kind int

const (
	KindFile Kind = iota
	KindDir
	KindSymlink
	KindHardLink
	KindDevice
	KindFIFO
	KindOther
)

// Member describes one entry of an archive: an immutable snapshot, with no
// I/O. Its content is read through the archive it came from.
type Member struct {
	object.OpaqueBase
	// Name is the name as stored, without the trailing "/" of a directory
	// entry. Raw keeps its bytes; Display is it with invalid UTF-8 shown
	// as U+FFFD.
	Raw     string
	Display string
	Size    int64
	ModTime time.Time
	Mode    int64
	Kind    Kind
	// Target is what a link points to; HasTarget is false unless the
	// member is a link whose target is known.
	Target    string
	HasTarget bool
	// Owner identifies the archive the Member came from, so a Reader can
	// tell its own Members from another's.
	Owner any
	// Pos is the member's position in its archive.
	Pos int
}

var _ object.Object = (*Member)(nil)

// NewMember builds a Member from the name as stored. One trailing "/" is
// removed.
func NewMember(owner any, name string, kind Kind, size int64, modTime time.Time, mode int64) *Member {
	raw := strings.TrimSuffix(name, "/")
	if raw == "" && name == "/" {
		raw = "/"
	}
	if kind == KindDir || kind == KindSymlink || kind == KindHardLink {
		size = 0
	}
	return &Member{
		OpaqueBase: object.MakeOpaqueBase("Member"),
		Raw:        raw,
		Display:    strings.ToValidUTF8(raw, "�"),
		Size:       size,
		ModTime:    modTime,
		Mode:       mode,
		Kind:       kind,
		Owner:      owner,
	}
}

func (m *Member) Constructor() *object.Function { return MemberType.Function }

func (m *Member) String() string            { return "<Member " + m.Display + ">" }
func (m *Member) ToString() (string, error) { return m.String(), nil }

func (m *Member) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Member)
	return ok && v == m, nil
}

func (m *Member) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.String(m.Display), nil
	case "size":
		return object.Integer(m.Size), nil
	case "modified_at":
		return timevalue.NewTime(m.ModTime), nil
	case "mode":
		return object.Integer(m.Mode), nil
	case "is_file":
		return object.Bool(m.Kind == KindFile), nil
	case "is_dir":
		return object.Bool(m.Kind == KindDir), nil
	case "is_symlink":
		return object.Bool(m.Kind == KindSymlink), nil
	case "link_target":
		if !m.HasTarget {
			return object.Nil, nil
		}
		return object.String(strings.ToValidUTF8(m.Target, "�")), nil
	}
	return nil, object.NewAttributeError("Member has no attribute '%s'", name)
}

func (m *Member) Attributes() []string {
	return []string{"name", "size", "modified_at", "mode", "is_file", "is_dir", "is_symlink", "link_target"}
}

// IsLink reports whether the member is a symbolic or hard link.
func (m *Member) IsLink() bool { return m.Kind == KindSymlink || m.Kind == KindHardLink }

// NotContent is the ValueError for open(member) on a member that has no
// content to read, or "" when it has.
func (m *Member) NotContent(fn string) error {
	switch m.Kind {
	case KindFile:
		return nil
	case KindDir:
		return object.NewValueError("%s() member %s is a directory", fn, Quote(m.Display))
	case KindSymlink, KindHardLink:
		return object.NewValueError("%s() member %s is a link", fn, Quote(m.Display))
	}
	return object.NewValueError("%s() member %s is not a regular file", fn, Quote(m.Display))
}

// Quote is how messages name a member or an archive.
func Quote(s string) string { return `"` + s + `"` }
