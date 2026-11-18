package archive

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/object"
)

// SafeName reports whether a member name, with "/" between components, can
// be written under a directory without leaving it: it is not empty, not
// absolute, has no ".." component, contains no "\" or NUL, and is local on
// this platform (which on Windows also rules out reserved device names).
// The same names are refused on every platform, so an archive unpacks the
// same way everywhere.
func SafeName(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\\x00") {
		return false
	}
	if strings.HasPrefix(name, "/") || hasDrive(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return filepath.IsLocal(filepath.FromSlash(name))
}

// hasDrive reports whether name starts with a Windows drive letter ("C:").
func hasDrive(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	c := name[0]
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// LinkTargetSafe reports whether a symbolic link stored at name pointing at
// target stays inside the directory it is unpacked into: the target is
// relative and, followed from where the link lands, names a place inside.
func LinkTargetSafe(name, target string) bool {
	if target == "" || strings.HasPrefix(target, "/") || hasDrive(target) || strings.ContainsAny(target, "\\\x00") {
		return false
	}
	return SafeName(path.Clean(path.Join(path.Dir(name), target)))
}

// WriterName checks a name given to a Writer and returns it as stored: the
// relative path written on this platform, cleaned, with "/" between its
// components. A Writer cannot store a name extract would refuse.
func WriterName(fn, given string) (string, error) {
	slashed := filepath.ToSlash(given)
	for _, part := range strings.Split(slashed, "/") {
		if part == ".." {
			return "", unusableName(fn, given)
		}
	}
	name := path.Clean(slashed)
	if name == "." || !SafeName(name) {
		return "", unusableName(fn, given)
	}
	return name, nil
}

func unusableName(fn, given string) error {
	return object.NewValueError("%s() name %s would be unpacked outside its directory", fn, Quote(given))
}

// LandingName turns the name= given to extract (a String with "/", or a
// Path written for this platform) into a member name with "/".
func LandingName(given string) string { return filepath.ToSlash(given) }
