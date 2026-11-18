package archive

import (
	"errors"
	"testing"
	"time"

	"github.com/aisk/goblin/object"
)

func TestSafeName(t *testing.T) {
	cases := map[string]bool{
		"a.txt":            true,
		"docs/guide.md":    true,
		".":                true,
		"./a":              true,
		"a/./b":            true,
		"":                 false,
		"../x":             false,
		"a/../b":           false,
		"a/..":             false,
		"/etc/passwd":      false,
		"C:/x":             false,
		`C:\x`:             false,
		`a\b`:              false,
		"a\x00b":           false,
		"../../etc/passwd": false,
	}
	for name, want := range cases {
		if got := SafeName(name); got != want {
			t.Errorf("SafeName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestLinkTargetSafe(t *testing.T) {
	cases := []struct {
		name, target string
		want         bool
	}{
		{"current", "releases/1.4", true},
		{"lib/libx.so", "libx.so.1", true},
		{"lib/up", "../bin", true},
		{"current", "/etc", false},
		{"current", "..", false},
		{"lib/up", "../../x", false},
		{"a", "", false},
		{"a", `..\x`, false},
	}
	for _, c := range cases {
		if got := LinkTargetSafe(c.name, c.target); got != c.want {
			t.Errorf("LinkTargetSafe(%q, %q) = %v, want %v", c.name, c.target, got, c.want)
		}
	}
}

func TestWriterName(t *testing.T) {
	good := map[string]string{
		"a.txt":       "a.txt",
		"dir//b/./c":  "dir/b/c",
		"site/":       "site",
		"project-1/x": "project-1/x",
	}
	for given, want := range good {
		got, err := WriterName("add", given)
		if err != nil || got != want {
			t.Errorf("WriterName(%q) = %q, %v; want %q", given, got, err, want)
		}
	}
	for _, given := range []string{"", ".", "../x", "a/../b", "/abs", `a\b`, "a\x00"} {
		_, err := WriterName("add", given)
		if !errors.Is(err, object.ValueError) {
			t.Errorf("WriterName(%q) error = %v, want ValueError", given, err)
		}
	}
	_, err := WriterName("add", "../x")
	if err.Error() != `add() name "../x" would be unpacked outside its directory` {
		t.Errorf("message = %q", err)
	}
}

func TestMemberProperties(t *testing.T) {
	m := NewMember(nil, "docs/\xffguide.md", KindFile, 12, timeZero, 0o640)
	if m.String() != "<Member docs/\uFFFDguide.md>" {
		t.Errorf("shows as %s", m)
	}
	if v, _ := m.GetAttr("link_target"); v != object.Nil {
		t.Errorf("link_target = %v", v)
	}
	dir := NewMember(nil, "docs/", KindDir, 99, timeZero, 0o755)
	if v, _ := dir.GetAttr("name"); v != object.String("docs") {
		t.Errorf("dir name = %v", v)
	}
	if v, _ := dir.GetAttr("size"); v != object.Integer(0) {
		t.Errorf("dir size = %v", v)
	}
	if _, err := MemberType.Function.Call(object.CallArgs{}); err == nil ||
		err.Error() != "Member cannot be constructed; iterate an archive from zip.open(path) or tar.open(path)" {
		t.Errorf("Member() error = %v", err)
	}
	if eq, _ := object.Equals(m, NewMember(nil, "docs/\xffguide.md", KindFile, 12, timeZero, 0o640)); eq {
		t.Error("two Members with the same fields are equal")
	}
	if !errors.Is(object.NewKindError(UnsafeError, "x"), object.BaseError) {
		t.Error("UnsafeError is not an Error")
	}
}

var timeZero = time.Time{}
