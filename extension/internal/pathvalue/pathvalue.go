// Package pathvalue holds the Path value type and the path argument helper
// shared by the modules that name files (fs, os, exec).
package pathvalue

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/object"
)

// Path is the name of a file: an immutable, cleaned path text. It performs
// no I/O; the fs module's functions act on what it names.
type Path struct {
	object.OpaqueBase
	raw string
}

var _ object.Object = (*Path)(nil)
var _ object.PathValue = (*Path)(nil)

// Type is the one constructor identity of Path, exported as fs.Path.
var Type = object.NewNativeConstructor("Path", construct)

// NewPath returns a Path holding the cleaned form of s. An empty string
// becomes ".", matching filepath.Clean.
func NewPath(s string) *Path {
	return &Path{OpaqueBase: object.MakeOpaqueBase("Path"), raw: filepath.Clean(s)}
}

func construct(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("Path", args)
	text := p.Path("text")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return NewPath(text), nil
}

// PathArg is object.PathString for a value that is not bound through an
// ArgParser (a list element, a variadic segment): anything other than a
// String or Path raises the standard TypeError for argument argName of fnName.
func PathArg(fnName, argName string, arg object.Object) (string, error) {
	if path, ok := object.PathString(arg); ok {
		return path, nil
	}
	return "", object.NewTypeError("%s() argument '%s' must be String or Path, got %s", fnName, argName, arg.TypeName())
}

// FilePath satisfies object.PathValue, so ArgParser.Path accepts a Path.
func (p *Path) FilePath() string { return p.raw }

func (p *Path) String() string            { return p.raw }
func (p *Path) ToString() (string, error) { return p.raw, nil }

// Equals compares the cleaned text; a Path never equals a String.
func (p *Path) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Path)
	return ok && p.raw == v.raw, nil
}

// Hash agrees with Equals.
func (p *Path) Hash() (uint64, error) { return object.String(p.raw).Hash() }

// Compare orders paths by their text.
func (p *Path) Compare(other object.Object) (int, error) {
	v, ok := other.(*Path)
	if !ok {
		return 0, object.NewOperandError(object.OpCompare, "Path", other)
	}
	return strings.Compare(p.raw, v.raw), nil
}

// Constructor satisfies object.Constructed, so type_of(p) is fs.Path.
func (p *Path) Constructor() *object.Function { return Type.Function }

// name is the last component, "" for a root and for ".".
func (p *Path) name() string {
	vol := filepath.VolumeName(p.raw)
	rest := p.raw[len(vol):]
	if rest == "" || rest == "." || (len(rest) == 1 && os.IsPathSeparator(rest[0])) {
		return ""
	}
	return filepath.Base(p.raw)
}

// suffix is the name from its last dot, unless that dot starts the name or
// the name is "..".
func suffixOf(name string) string {
	if name == ".." {
		return ""
	}
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return ""
	}
	return name[i:]
}

func (p *Path) GetAttr(name string) (object.Object, error) {
	switch name {
	case "name":
		return object.String(p.name()), nil
	case "stem":
		n := p.name()
		return object.String(n[:len(n)-len(suffixOf(n))]), nil
	case "suffix":
		return object.String(suffixOf(p.name())), nil
	case "parent":
		return NewPath(filepath.Dir(p.raw)), nil
	case "is_absolute":
		return object.Bool(filepath.IsAbs(p.raw)), nil
	case "join":
		return &object.Function{Name: "join", Fn: p.join}, nil
	case "with_name":
		return &object.Function{Name: "with_name", Fn: p.withName}, nil
	case "with_suffix":
		return &object.Function{Name: "with_suffix", Fn: p.withSuffix}, nil
	case "relative_to":
		return &object.Function{Name: "relative_to", Fn: p.relativeTo}, nil
	}
	return nil, object.NewAttributeError("Path has no attribute '%s'", name)
}

func (p *Path) Attributes() []string {
	return []string{"name", "stem", "suffix", "parent", "is_absolute", "join", "with_name", "with_suffix", "relative_to"}
}

func (p *Path) join(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("join", args)
	segments := ap.Rest()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return p, nil
	}
	elems := make([]string, 0, len(segments)+1)
	elems = append(elems, p.raw)
	for _, arg := range segments {
		seg, err := PathArg("join", "segments", arg)
		if err != nil {
			return nil, err
		}
		elems = append(elems, seg)
	}
	return NewPath(filepath.Join(elems...)), nil
}

func hasSeparator(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 128 && os.IsPathSeparator(byte(r)) })
}

func (p *Path) withName(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("with_name", args)
	name := string(ap.Str("name"))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if name == "" || hasSeparator(name) {
		return nil, object.NewValueError("with_name() invalid name %q", name)
	}
	if p.name() == "" {
		return nil, object.NewValueError("with_name() path %q has no name", p.raw)
	}
	return NewPath(filepath.Join(filepath.Dir(p.raw), name)), nil
}

func (p *Path) withSuffix(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("with_suffix", args)
	suffix := string(ap.Str("suffix"))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if (suffix != "" && !strings.HasPrefix(suffix, ".")) || hasSeparator(suffix) {
		return nil, object.NewValueError("with_suffix() invalid suffix %q", suffix)
	}
	name := p.name()
	if name == "" {
		return nil, object.NewValueError("with_suffix() path %q has no name", p.raw)
	}
	stem := name[:len(name)-len(suffixOf(name))]
	return NewPath(filepath.Join(filepath.Dir(p.raw), stem+suffix)), nil
}

func (p *Path) relativeTo(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("relative_to", args)
	base := ap.Path("base")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, p.raw)
	if err != nil {
		return nil, object.NewValueError("relative_to() cannot make %q relative to %q", p.raw, filepath.Clean(base))
	}
	return NewPath(rel), nil
}
