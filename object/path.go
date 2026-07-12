package object

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Path is an object-oriented, OS-specific filesystem path, modelled after
// Python's pathlib.Path. It wraps a cleaned path string; pure operations
// (name, parent, joining via `/`, ...) never touch disk, while IO methods
// (exists, read_text, iterdir, ...) do.
type Path struct {
	raw string
}

var _ Object = (*Path)(nil)

// NewPath returns a Path holding the cleaned form of s. An empty string
// becomes ".", matching filepath.Clean.
func NewPath(s string) *Path {
	return &Path{raw: filepath.Clean(s)}
}

// PathString extracts a filesystem path string from a String or Path,
// mirroring Python's os.fspath(). Library functions use it as the single point
// where a "path-like" argument is accepted, so a Path can be passed anywhere a
// path string is expected.
func PathString(obj Object) (string, bool) {
	switch v := obj.(type) {
	case String:
		return string(v), true
	case *Path:
		return v.raw, true
	default:
		return "", false
	}
}

func (p *Path) String() string { return p.raw }

func (p *Path) ToString() (string, error) { return p.String(), nil }
func (p *Path) Bool() bool                { return p.raw != "" && p.raw != "." }

func (p *Path) Compare(other Object) (int, error) {
	v, ok := other.(*Path)
	if !ok {
		return 0, NewTypeError("cannot compare Path and %T", other)
	}
	switch {
	case p.raw < v.raw:
		return -1, nil
	case p.raw > v.raw:
		return 1, nil
	default:
		return 0, nil
	}
}

func (p *Path) Add(Object) (Object, error) { return nil, NewTypeError("cannot add Path") }
func (p *Path) Minus(Object) (Object, error) {
	return nil, NewTypeError("cannot subtract from Path")
}
func (p *Path) Multiply(Object) (Object, error) { return nil, NewTypeError("cannot multiply Path") }

// Divide overloads `/` as path joining, so `Path("/tmp") / "log" / "a.txt"`
// reads like a filesystem path.
func (p *Path) Divide(other Object) (Object, error) {
	seg, err := pathSegment("/", other)
	if err != nil {
		return nil, err
	}
	return NewPath(filepath.Join(p.raw, seg)), nil
}

func (p *Path) And(other Object) (Object, error) {
	return Bool(p.Bool() && other.Bool()), nil
}
func (p *Path) Or(other Object) (Object, error) {
	return Bool(p.Bool() || other.Bool()), nil
}
func (p *Path) Not() (Object, error) { return Bool(!p.Bool()), nil }
func (p *Path) Iter() ([]Object, error) {
	return nil, NewTypeError("Path is not iterable; use iterdir() to list a directory")
}
func (p *Path) Index(Object) (Object, error) { return nil, NewTypeError("Path is not indexable") }

func (p *Path) GetAttr(name string) (Object, error) {
	switch name {
	case "attributes":
		return AttributesFunction(p), nil
	// Pure properties — return values directly.
	case "name":
		return String(filepath.Base(p.raw)), nil
	case "stem":
		base := filepath.Base(p.raw)
		return String(strings.TrimSuffix(base, filepath.Ext(base))), nil
	case "suffix":
		return String(filepath.Ext(p.raw)), nil
	case "parent":
		return NewPath(filepath.Dir(p.raw)), nil
	case "parts":
		return &List{Elements: pathParts(p.raw)}, nil
	// Pure methods.
	case "is_absolute":
		return &Function{Name: "is_absolute", Fn: p.IsAbsolute}, nil
	case "with_name":
		return &Function{Name: "with_name", Fn: p.WithName}, nil
	case "with_suffix":
		return &Function{Name: "with_suffix", Fn: p.WithSuffix}, nil
	case "join":
		return &Function{Name: "join", Fn: p.Join}, nil
	case "relative_to":
		return &Function{Name: "relative_to", Fn: p.RelativeTo}, nil
	case "match":
		return &Function{Name: "match", Fn: p.Match}, nil
	case "as_posix":
		return &Function{Name: "as_posix", Fn: p.AsPosix}, nil
	// IO methods.
	case "exists":
		return &Function{Name: "exists", Fn: p.Exists}, nil
	case "is_dir":
		return &Function{Name: "is_dir", Fn: p.IsDir}, nil
	case "is_file":
		return &Function{Name: "is_file", Fn: p.IsFile}, nil
	case "is_symlink":
		return &Function{Name: "is_symlink", Fn: p.IsSymlink}, nil
	case "resolve":
		return &Function{Name: "resolve", Fn: p.Resolve}, nil
	case "read_text":
		return &Function{Name: "read_text", Fn: p.ReadText}, nil
	case "write_text":
		return &Function{Name: "write_text", Fn: p.WriteText}, nil
	case "iterdir":
		return &Function{Name: "iterdir", Fn: p.IterDir}, nil
	case "glob":
		return &Function{Name: "glob", Fn: p.Glob}, nil
	case "mkdir":
		return &Function{Name: "mkdir", Fn: p.Mkdir}, nil
	case "unlink":
		return &Function{Name: "unlink", Fn: p.Unlink}, nil
	case "rename":
		return &Function{Name: "rename", Fn: p.Rename}, nil
	case "constructor":
		return PathConstructorFn, nil
	}
	return nil, NewAttributeError("Path has no attribute '%s'", name)
}

func (p *Path) Attributes() []string {
	return []string{
		"attributes", "name", "stem", "suffix", "parent", "parts",
		"is_absolute", "with_name", "with_suffix", "join", "relative_to", "match", "as_posix",
		"exists", "is_dir", "is_file", "is_symlink", "resolve", "read_text", "write_text",
		"iterdir", "glob", "mkdir", "unlink", "rename", "constructor",
	}
}

// pathSegment coerces a String or Path argument into a plain path string.
func pathSegment(fnName string, arg Object) (string, error) {
	switch v := arg.(type) {
	case String:
		return string(v), nil
	case *Path:
		return v.raw, nil
	default:
		return "", NewTypeError("%s() argument must be a string or Path, got %T", fnName, arg)
	}
}

// pathParts splits raw into its components, keeping the leading anchor (a
// volume and/or root separator) as a single first element, like pathlib.
func pathParts(raw string) []Object {
	clean := filepath.Clean(raw)
	sep := string(filepath.Separator)
	vol := filepath.VolumeName(clean)
	rest := clean[len(vol):]

	var parts []Object
	switch {
	case strings.HasPrefix(rest, sep):
		parts = append(parts, String(vol+sep))
		rest = strings.TrimPrefix(rest, sep)
	case vol != "":
		parts = append(parts, String(vol))
	}
	if rest != "" && rest != "." {
		for _, seg := range strings.Split(rest, sep) {
			if seg != "" {
				parts = append(parts, String(seg))
			}
		}
	}
	return parts
}

func (p *Path) IsAbsolute(args CallArgs) (Object, error) {
	if err := requireNoArgs("is_absolute", args); err != nil {
		return nil, err
	}
	return Bool(filepath.IsAbs(p.raw)), nil
}

func (p *Path) WithName(args CallArgs) (Object, error) {
	ap := NewArgParser("with_name", args)
	name := ap.Str("name")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if name == "" || strings.ContainsRune(string(name), filepath.Separator) {
		return nil, NewValueError("with_name() invalid name: %q", string(name))
	}
	return NewPath(filepath.Join(filepath.Dir(p.raw), string(name))), nil
}

func (p *Path) WithSuffix(args CallArgs) (Object, error) {
	ap := NewArgParser("with_suffix", args)
	suffix := ap.Str("suffix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if suffix != "" && !strings.HasPrefix(string(suffix), ".") {
		return nil, NewValueError("with_suffix() invalid suffix: %q", string(suffix))
	}
	stem := strings.TrimSuffix(p.raw, filepath.Ext(p.raw))
	return NewPath(stem + string(suffix)), nil
}

func (p *Path) Join(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("join", args); err != nil {
		return nil, err
	}
	elems := make([]string, 0, len(args.Positional)+1)
	elems = append(elems, p.raw)
	for _, arg := range args.Positional {
		seg, err := pathSegment("join", arg)
		if err != nil {
			return nil, err
		}
		elems = append(elems, seg)
	}
	return NewPath(filepath.Join(elems...)), nil
}

func (p *Path) RelativeTo(args CallArgs) (Object, error) {
	ap := NewArgParser("relative_to", args)
	other := ap.Any("other")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	base, err := pathSegment("relative_to", other)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, p.raw)
	if err != nil {
		return nil, WrapError(ValueError, "relative_to() failed", err)
	}
	return NewPath(rel), nil
}

func (p *Path) Match(args CallArgs) (Object, error) {
	ap := NewArgParser("match", args)
	pattern := ap.Str("pattern")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	matched, err := filepath.Match(string(pattern), filepath.Base(p.raw))
	if err != nil {
		return nil, WrapError(ParseError, "match() failed", err)
	}
	return Bool(matched), nil
}

func (p *Path) AsPosix(args CallArgs) (Object, error) {
	if err := requireNoArgs("as_posix", args); err != nil {
		return nil, err
	}
	return String(filepath.ToSlash(p.raw)), nil
}

func (p *Path) Exists(args CallArgs) (Object, error) {
	if err := requireNoArgs("exists", args); err != nil {
		return nil, err
	}
	_, err := os.Stat(p.raw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return False, nil
		}
		return nil, WrapNativeError(IOError, "exists() failed", err)
	}
	return True, nil
}

func (p *Path) IsDir(args CallArgs) (Object, error) {
	if err := requireNoArgs("is_dir", args); err != nil {
		return nil, err
	}
	return p.statMode("is_dir", func(m fs.FileMode) bool { return m.IsDir() })
}

func (p *Path) IsFile(args CallArgs) (Object, error) {
	if err := requireNoArgs("is_file", args); err != nil {
		return nil, err
	}
	return p.statMode("is_file", func(m fs.FileMode) bool { return m.IsRegular() })
}

func (p *Path) IsSymlink(args CallArgs) (Object, error) {
	if err := requireNoArgs("is_symlink", args); err != nil {
		return nil, err
	}
	info, err := os.Lstat(p.raw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return False, nil
		}
		return nil, WrapNativeError(IOError, "is_symlink() failed", err)
	}
	return Bool(info.Mode()&fs.ModeSymlink != 0), nil
}

// statMode stats the path and reports pred(mode), treating a missing path as
// false to match pathlib's is_dir()/is_file() semantics.
func (p *Path) statMode(fnName string, pred func(fs.FileMode) bool) (Object, error) {
	info, err := os.Stat(p.raw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return False, nil
		}
		return nil, WrapNativeError(IOError, fnName+"() failed", err)
	}
	return Bool(pred(info.Mode())), nil
}

func (p *Path) Resolve(args CallArgs) (Object, error) {
	if err := requireNoArgs("resolve", args); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(p.raw)
	if err != nil {
		return nil, WrapNativeError(IOError, "resolve() failed", err)
	}
	// Follow symlinks when the target exists; otherwise fall back to the
	// absolute path, like pathlib.resolve(strict=False).
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return NewPath(resolved), nil
	}
	return NewPath(abs), nil
}

func (p *Path) ReadText(args CallArgs) (Object, error) {
	if err := requireNoArgs("read_text", args); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p.raw)
	if err != nil {
		return nil, WrapNativeError(IOError, "read_text() failed", err)
	}
	return String(data), nil
}

func (p *Path) WriteText(args CallArgs) (Object, error) {
	ap := NewArgParser("write_text", args)
	data := ap.Str("data")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p.raw, []byte(data), 0644); err != nil {
		return nil, WrapNativeError(IOError, "write_text() failed", err)
	}
	return Nil, nil
}

func (p *Path) IterDir(args CallArgs) (Object, error) {
	if err := requireNoArgs("iterdir", args); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p.raw)
	if err != nil {
		return nil, WrapNativeError(IOError, "iterdir() failed", err)
	}
	elements := make([]Object, len(entries))
	for i, entry := range entries {
		elements[i] = NewPath(filepath.Join(p.raw, entry.Name()))
	}
	return &List{Elements: elements}, nil
}

func (p *Path) Glob(args CallArgs) (Object, error) {
	ap := NewArgParser("glob", args)
	pattern := ap.Str("pattern")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(p.raw, string(pattern)))
	if err != nil {
		return nil, WrapError(ParseError, "glob() failed", err)
	}
	elements := make([]Object, len(matches))
	for i, m := range matches {
		elements[i] = NewPath(m)
	}
	return &List{Elements: elements}, nil
}

func (p *Path) Mkdir(args CallArgs) (Object, error) {
	ap := NewArgParser("mkdir", args)
	parents := bool(ap.BoolOr("parents", false))
	existOk := bool(ap.BoolOr("exist_ok", false))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	var err error
	if parents {
		err = os.MkdirAll(p.raw, 0755)
	} else {
		err = os.Mkdir(p.raw, 0755)
	}
	if err != nil {
		if existOk && errors.Is(err, fs.ErrExist) {
			return Nil, nil
		}
		return nil, WrapNativeError(IOError, "mkdir() failed", err)
	}
	return Nil, nil
}

func (p *Path) Unlink(args CallArgs) (Object, error) {
	if err := requireNoArgs("unlink", args); err != nil {
		return nil, err
	}
	if err := os.Remove(p.raw); err != nil {
		return nil, WrapNativeError(IOError, "unlink() failed", err)
	}
	return Nil, nil
}

func (p *Path) Rename(args CallArgs) (Object, error) {
	ap := NewArgParser("rename", args)
	target := ap.Any("target")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	targetPath, err := pathSegment("rename", target)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(p.raw, targetPath); err != nil {
		return nil, WrapNativeError(IOError, "rename() failed", err)
	}
	return NewPath(targetPath), nil
}

// requireNoArgs rejects any positional or keyword arguments for the zero-arg
// query methods.
func requireNoArgs(fnName string, args CallArgs) error {
	if err := RequireNoKeyword(fnName, args); err != nil {
		return err
	}
	if len(args.Positional) != 0 {
		return NewTypeError("%s() takes no arguments, got %d", fnName, len(args.Positional))
	}
	return nil
}

// PathConstructorFn builds a Path from zero or more string/Path segments,
// joining them; with no arguments it yields Path("."). Exposed as `path.Path`.
var PathConstructorFn = &Function{Name: "Path", Fn: PathConstructor}

func PathConstructor(args CallArgs) (Object, error) {
	if err := RequireNoKeyword("Path", args); err != nil {
		return nil, err
	}
	if len(args.Positional) == 0 {
		return NewPath("."), nil
	}
	segs := make([]string, len(args.Positional))
	for i, arg := range args.Positional {
		seg, err := pathSegment("Path", arg)
		if err != nil {
			return nil, err
		}
		segs[i] = seg
	}
	return NewPath(filepath.Join(segs...)), nil
}
