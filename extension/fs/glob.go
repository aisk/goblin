package fs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

func glob(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("glob", args)
	pattern := p.Path("pattern")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	root, components := splitPattern(pattern)
	for _, c := range components {
		if _, err := filepath.Match(c, ""); err != nil {
			return nil, object.NewValueError("glob() malformed pattern %q", pattern)
		}
	}
	g := &globber{seen: map[string]bool{}}
	if err := g.match(root, components); err != nil {
		return nil, err
	}
	sort.Strings(g.found)
	items := make([]object.Object, len(g.found))
	for i, path := range g.found {
		items[i] = pathvalue.NewPath(path)
	}
	return &object.List{Elements: items}, nil
}

// splitPattern separates the directory a pattern starts from (its root, or
// "" for the working directory) from its non-empty components.
func splitPattern(pattern string) (string, []string) {
	vol := filepath.VolumeName(pattern)
	rest := pattern[len(vol):]
	root := vol
	if rest != "" && os.IsPathSeparator(rest[0]) {
		root += string(filepath.Separator)
	}
	var components []string
	for _, c := range strings.FieldsFunc(rest, func(r rune) bool { return r < 128 && os.IsPathSeparator(uint8(r)) }) {
		components = append(components, c)
	}
	return root, components
}

type globber struct {
	found []string
	seen  map[string]bool
}

func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return filepath.Join(dir, name)
}

func (g *globber) add(path string) {
	if path == "" {
		path = "."
	}
	path = filepath.Clean(path)
	if !g.seen[path] {
		g.seen[path] = true
		g.found = append(g.found, path)
	}
}

// list reads a directory for matching; one that is missing or is not a
// directory has no entries.
func list(dir string) ([]os.DirEntry, error) {
	name := dir
	if name == "" {
		name = "."
	}
	entries, err := os.ReadDir(name)
	if err != nil {
		if notThere(err) {
			return nil, nil
		}
		return nil, fail("glob", "cannot read", name, err)
	}
	return entries, nil
}

func (g *globber) match(dir string, components []string) error {
	if len(components) == 0 {
		g.add(dir)
		return nil
	}
	c, rest := components[0], components[1:]
	if c == "**" {
		// Zero directories, then each subdirectory with ** still pending.
		// Symlinked directories are not followed, so a cycle cannot recur.
		if err := g.match(dir, rest); err != nil {
			return err
		}
		entries, err := list(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				if err := g.match(join(dir, e.Name()), components); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if !hasMeta(c) {
		path := join(dir, c)
		if _, err := os.Lstat(path); err != nil {
			if notThere(err) {
				return nil
			}
			return fail("glob", "cannot stat", path, err)
		}
		return g.match(path, rest)
	}
	entries, err := list(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		ok, _ := filepath.Match(c, e.Name())
		if ok {
			if err := g.match(join(dir, e.Name()), rest); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasMeta(c string) bool {
	return strings.ContainsAny(c, `*?[`) || (filepath.Separator != '\\' && strings.Contains(c, `\`))
}
