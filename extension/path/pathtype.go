package path

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/object"
)

// Path is an object-oriented, OS-specific filesystem path, modelled after
// Python's pathlib.Path. It wraps a cleaned path string; pure operations
// (name, parent, joining via `/`, ...) never touch disk, while IO methods
// (exists, read_text, iterdir, ...) do.
type Path struct {
	object.NoReflectedOps
	object.NoAssignment
	raw string
}

var _ object.Object = (*Path)(nil)

// NewPath returns a Path holding the cleaned form of s. An empty string
// becomes ".", matching filepath.Clean.
func NewPath(s string) *Path {
	return &Path{raw: filepath.Clean(s)}
}

// PathString extracts a filesystem path string from a String or Path,
// mirroring Python's os.fspath(). Library functions use it as the single point
// where a "path-like" argument is accepted, so a Path can be passed anywhere a
// path string is expected.
func PathString(obj object.Object) (string, bool) {
	switch v := obj.(type) {
	case object.String:
		return string(v), true
	case *Path:
		return v.raw, true
	default:
		return "", false
	}
}

func (p *Path) TypeName() string { return "Path" }

func (p *Path) String() string { return p.raw }

func (p *Path) ToString() (string, error) { return p.String(), nil }
func (p *Path) ToBool() (bool, error)     { return p.raw != "" && p.raw != ".", nil }

func (p *Path) Equals(other object.Object) (bool, error) {
	v, ok := other.(*Path)
	return ok && p.raw == v.raw, nil
}

func (p *Path) Compare(other object.Object) (int, error) {
	v, ok := other.(*Path)
	if !ok {
		return 0, object.NewTypeError("cannot compare Path and %s", other.TypeName())
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

func (p *Path) Add(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot add Path")
}
func (p *Path) Minus(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot subtract from Path")
}
func (p *Path) Multiply(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot multiply Path")
}

// Divide overloads `/` as path joining, so `Path("/tmp") / "log" / "a.txt"`
// reads like a filesystem path.
func (p *Path) Divide(other object.Object) (object.Object, error) {
	seg, err := pathSegment("/", "other", other)
	if err != nil {
		return nil, object.NewTypeError("cannot join Path with %s using /", other.TypeName())
	}
	return NewPath(filepath.Join(p.raw, seg)), nil
}

func (p *Path) Modulo(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("cannot modulo Path")
}

func (p *Path) Not() (object.Object, error) {
	return object.Bool(p.raw == "" || p.raw == "."), nil
}
func (p *Path) Iter() ([]object.Object, error) {
	return nil, object.NewTypeError("Path is not iterable; use iterdir() to list a directory")
}
func (p *Path) Index(object.Object) (object.Object, error) {
	return nil, object.NewTypeError("Path is not indexable")
}

func (p *Path) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(p), nil
	// Pure properties — return values directly.
	case "name":
		return object.String(filepath.Base(p.raw)), nil
	case "stem":
		base := filepath.Base(p.raw)
		return object.String(strings.TrimSuffix(base, filepath.Ext(base))), nil
	case "suffix":
		return object.String(filepath.Ext(p.raw)), nil
	case "parent":
		return NewPath(filepath.Dir(p.raw)), nil
	case "parts":
		return &object.List{Elements: pathParts(p.raw)}, nil
	// Pure methods.
	case "is_absolute":
		return &object.Function{Name: "is_absolute", Fn: p.IsAbsolute}, nil
	case "with_name":
		return &object.Function{Name: "with_name", Fn: p.WithName}, nil
	case "with_suffix":
		return &object.Function{Name: "with_suffix", Fn: p.WithSuffix}, nil
	case "join":
		return &object.Function{Name: "join", Fn: p.Join}, nil
	case "relative_to":
		return &object.Function{Name: "relative_to", Fn: p.RelativeTo}, nil
	case "match":
		return &object.Function{Name: "match", Fn: p.Match}, nil
	case "as_posix":
		return &object.Function{Name: "as_posix", Fn: p.AsPosix}, nil
	// IO methods.
	case "exists":
		return &object.Function{Name: "exists", Fn: p.Exists}, nil
	case "is_dir":
		return &object.Function{Name: "is_dir", Fn: p.IsDir}, nil
	case "is_file":
		return &object.Function{Name: "is_file", Fn: p.IsFile}, nil
	case "is_symlink":
		return &object.Function{Name: "is_symlink", Fn: p.IsSymlink}, nil
	case "resolve":
		return &object.Function{Name: "resolve", Fn: p.Resolve}, nil
	case "read_text":
		return &object.Function{Name: "read_text", Fn: p.ReadText}, nil
	case "read_bytes":
		return &object.Function{Name: "read_bytes", Fn: p.ReadBytes}, nil
	case "write_text":
		return &object.Function{Name: "write_text", Fn: p.WriteText}, nil
	case "write_bytes":
		return &object.Function{Name: "write_bytes", Fn: p.WriteBytes}, nil
	case "iterdir":
		return &object.Function{Name: "iterdir", Fn: p.IterDir}, nil
	case "glob":
		return &object.Function{Name: "glob", Fn: p.Glob}, nil
	case "mkdir":
		return &object.Function{Name: "mkdir", Fn: p.Mkdir}, nil
	case "unlink":
		return &object.Function{Name: "unlink", Fn: p.Unlink}, nil
	case "rename":
		return &object.Function{Name: "rename", Fn: p.Rename}, nil
	case "constructor":
		return PathConstructorFn, nil
	}
	return nil, object.NewAttributeError("Path has no attribute '%s'", name)
}

func (p *Path) Attributes() []string {
	return []string{
		"attributes", "name", "stem", "suffix", "parent", "parts",
		"is_absolute", "with_name", "with_suffix", "join", "relative_to", "match", "as_posix",
		"exists", "is_dir", "is_file", "is_symlink", "resolve",
		"read_text", "read_bytes", "write_text", "write_bytes",
		"iterdir", "glob", "mkdir", "unlink", "rename", "constructor",
	}
}

// pathSegment coerces a String or Path argument into a plain path string.
func pathSegment(fnName, argName string, arg object.Object) (string, error) {
	switch v := arg.(type) {
	case object.String:
		return string(v), nil
	case *Path:
		return v.raw, nil
	default:
		return "", object.NewTypeError("%s() argument '%s' must be str or Path, got %s", fnName, argName, arg.TypeName())
	}
}

// pathParts splits raw into its components, keeping the leading anchor (a
// volume and/or root separator) as a single first element, like pathlib.
func pathParts(raw string) []object.Object {
	clean := filepath.Clean(raw)
	sep := string(filepath.Separator)
	vol := filepath.VolumeName(clean)
	rest := clean[len(vol):]

	var parts []object.Object
	switch {
	case strings.HasPrefix(rest, sep):
		parts = append(parts, object.String(vol+sep))
		rest = strings.TrimPrefix(rest, sep)
	case vol != "":
		parts = append(parts, object.String(vol))
	}
	if rest != "" && rest != "." {
		for _, seg := range strings.Split(rest, sep) {
			if seg != "" {
				parts = append(parts, object.String(seg))
			}
		}
	}
	return parts
}

func (p *Path) IsAbsolute(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("is_absolute", args); err != nil {
		return nil, err
	}
	return object.Bool(filepath.IsAbs(p.raw)), nil
}

func (p *Path) WithName(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("with_name", args)
	name := ap.Str("name")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if name == "" || strings.ContainsRune(string(name), filepath.Separator) {
		return nil, object.NewValueError("with_name() invalid name: %q", string(name))
	}
	return NewPath(filepath.Join(filepath.Dir(p.raw), string(name))), nil
}

func (p *Path) WithSuffix(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("with_suffix", args)
	suffix := ap.Str("suffix")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if suffix != "" && !strings.HasPrefix(string(suffix), ".") {
		return nil, object.NewValueError("with_suffix() invalid suffix: %q", string(suffix))
	}
	stem := strings.TrimSuffix(p.raw, filepath.Ext(p.raw))
	return NewPath(stem + string(suffix)), nil
}

func (p *Path) Join(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("join", args)
	segments := ap.Rest()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	elems := make([]string, 0, len(segments)+1)
	elems = append(elems, p.raw)
	for _, arg := range segments {
		seg, err := pathSegment("join", "segments", arg)
		if err != nil {
			return nil, err
		}
		elems = append(elems, seg)
	}
	return NewPath(filepath.Join(elems...)), nil
}

func (p *Path) RelativeTo(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("relative_to", args)
	other := ap.Any("other")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	base, err := pathSegment("relative_to", "other", other)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, p.raw)
	if err != nil {
		return nil, object.WrapError(object.ValueError, "relative_to() cannot make path relative to the base", err)
	}
	return NewPath(rel), nil
}

func (p *Path) Match(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("match", args)
	pattern := ap.Str("pattern")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	matched, err := filepath.Match(string(pattern), filepath.Base(p.raw))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "match() invalid pattern", err)
	}
	return object.Bool(matched), nil
}

func (p *Path) AsPosix(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("as_posix", args); err != nil {
		return nil, err
	}
	return object.String(filepath.ToSlash(p.raw)), nil
}

func (p *Path) Exists(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("exists", args); err != nil {
		return nil, err
	}
	_, err := os.Stat(p.raw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return object.False, nil
		}
		return nil, object.WrapNativeError(object.IOError, "exists() failed to stat path", err)
	}
	return object.True, nil
}

func (p *Path) IsDir(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("is_dir", args); err != nil {
		return nil, err
	}
	return p.statMode("is_dir", func(m fs.FileMode) bool { return m.IsDir() })
}

func (p *Path) IsFile(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("is_file", args); err != nil {
		return nil, err
	}
	return p.statMode("is_file", func(m fs.FileMode) bool { return m.IsRegular() })
}

func (p *Path) IsSymlink(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("is_symlink", args); err != nil {
		return nil, err
	}
	info, err := os.Lstat(p.raw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return object.False, nil
		}
		return nil, object.WrapNativeError(object.IOError, "is_symlink() failed to stat path", err)
	}
	return object.Bool(info.Mode()&fs.ModeSymlink != 0), nil
}

// statMode stats the path and reports pred(mode), treating a missing path as
// false to match pathlib's is_dir()/is_file() semantics.
func (p *Path) statMode(fnName string, pred func(fs.FileMode) bool) (object.Object, error) {
	info, err := os.Stat(p.raw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return object.False, nil
		}
		return nil, object.WrapNativeError(object.IOError, fnName+"() failed to stat path", err)
	}
	return object.Bool(pred(info.Mode())), nil
}

func (p *Path) Resolve(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("resolve", args); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(p.raw)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "resolve() failed to resolve path", err)
	}
	// Follow symlinks when the target exists; otherwise fall back to the
	// absolute path, like pathlib.resolve(strict=False).
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return NewPath(resolved), nil
	}
	return NewPath(abs), nil
}

func (p *Path) ReadText(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("read_text", args); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p.raw)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "read_text() failed to read file", err)
	}
	return object.String(data), nil
}

func (p *Path) ReadBytes(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("read_bytes", args); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p.raw)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "read_bytes() failed to read file", err)
	}
	return object.NewBytes(data), nil
}

func (p *Path) WriteText(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("write_text", args)
	data := ap.Str("data")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p.raw, []byte(data), 0644); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write_text() failed to write file", err)
	}
	return object.Nil, nil
}

func (p *Path) WriteBytes(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("write_bytes", args)
	data := ap.Any("data")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	b, ok := data.(object.Bytes)
	if !ok {
		return nil, object.NewTypeError("write_bytes() argument 'data' must be Bytes, got %s", data.TypeName())
	}
	if err := os.WriteFile(p.raw, []byte(b), 0644); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write_bytes() failed to write file", err)
	}
	return object.Nil, nil
}

func (p *Path) IterDir(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("iterdir", args); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p.raw)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "iterdir() failed to read directory", err)
	}
	elements := make([]object.Object, len(entries))
	for i, entry := range entries {
		elements[i] = NewPath(filepath.Join(p.raw, entry.Name()))
	}
	return &object.List{Elements: elements}, nil
}

func (p *Path) Glob(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("glob", args)
	pattern := ap.Str("pattern")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(p.raw, string(pattern)))
	if err != nil {
		return nil, object.WrapError(object.ParseError, "glob() invalid pattern", err)
	}
	elements := make([]object.Object, len(matches))
	for i, m := range matches {
		elements[i] = NewPath(m)
	}
	return &object.List{Elements: elements}, nil
}

func (p *Path) Mkdir(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("mkdir", args)
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
			return object.Nil, nil
		}
		return nil, object.WrapNativeError(object.IOError, "mkdir() failed to create directory", err)
	}
	return object.Nil, nil
}

func (p *Path) Unlink(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("unlink", args); err != nil {
		return nil, err
	}
	if err := os.Remove(p.raw); err != nil {
		return nil, object.WrapNativeError(object.IOError, "unlink() failed to remove path", err)
	}
	return object.Nil, nil
}

func (p *Path) Rename(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("rename", args)
	target := ap.Any("target")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	targetPath, err := pathSegment("rename", "target", target)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(p.raw, targetPath); err != nil {
		return nil, object.WrapNativeError(object.IOError, "rename() failed to rename path", err)
	}
	return NewPath(targetPath), nil
}

// PathConstructorFn builds a Path from zero or more string/Path segments,
// joining them; with no arguments it yields Path("."). Exposed as `path.Path`.
var PathConstructorFn = &object.Function{Name: "Path", Fn: PathConstructor}

func PathConstructor(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Path", args)
	segments := ap.Rest()
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return NewPath("."), nil
	}
	segs := make([]string, len(segments))
	for i, arg := range segments {
		seg, err := pathSegment("Path", "segments", arg)
		if err != nil {
			return nil, err
		}
		segs[i] = seg
	}
	return NewPath(filepath.Join(segs...)), nil
}
