package archive

import (
	"io"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

// Content reads one member's content for the File open(member) returns. It
// stops working when Alive reports false (the archive was closed, or a tar
// moved past the member), and reports damage that Corrupt recognises as a
// ParseError naming the call that reached it.
type Content struct {
	R           io.Reader
	Alive       func() bool
	Member      *Member
	Archive     string
	ArchivePath object.Object
	Corrupt     func(err error) bool
}

func (c *Content) Read(p []byte) (int, error) {
	if !c.Alive() {
		return 0, goneError{name: c.Member.Display}
	}
	n, err := c.R.Read(p)
	if err != nil && err != io.EOF && c.Corrupt != nil && c.Corrupt(err) {
		return n, corruptError{c: c}
	}
	return n, err
}

// MemberFile wraps content as the File open(member) returns: it has no
// path and shows as <File bundle.zip:manifest.json>. It holds nothing of
// its own, so its close() releases nothing.
func MemberFile(c *Content) *filestream.File {
	label := c.Member.Display
	if c.Archive != "" {
		label = c.Archive + ":" + label
	}
	return filestream.NewReader(label, c, nil)
}

// goneError is reading a member File whose archive was closed or, in a
// tar, moved to the next member.
type goneError struct{ name string }

func (e goneError) Error() string { return "member " + Quote(e.name) + " is no longer open" }

func (e goneError) ForMethod(method string) error {
	if method == "" {
		return object.NewValueError("member %s is no longer open", Quote(e.name))
	}
	return object.NewValueError("%s() member %s is no longer open", method, Quote(e.name))
}

// corruptError is damage found in a member's content.
type corruptError struct{ c *Content }

func (e corruptError) Error() string { return "corrupt data in member " + Quote(e.c.Member.Display) }

func (e corruptError) ForMethod(method string) error {
	return CorruptMember(method, e.c.Member, e.c.Archive, e.c.ArchivePath)
}

// CorruptMember is the ParseError for damage in member m, raised by the
// call method ("" for a loop).
func CorruptMember(method string, m *Member, archive string, archivePath object.Object) error {
	of := ""
	if archive != "" {
		of = " of " + Quote(archive)
	}
	var err *object.Error
	if method == "" {
		err = object.NewParseError("corrupt data in member %s%s", Quote(m.Display), of)
	} else {
		err = object.NewParseError("%s() corrupt data in member %s%s", method, Quote(m.Display), of)
	}
	if archivePath == nil {
		archivePath = object.Nil
	}
	return err.With("path", archivePath)
}
