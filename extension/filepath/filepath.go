package filepath

import (
	"fmt"
	stdpath "path/filepath"

	"github.com/aisk/goblin/object"
)

// Execute returns the filepath module, exposing Go's path/filepath functions.
func Execute() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"abs":           &object.Function{Name: "abs", Fn: abs},
			"base":          &object.Function{Name: "base", Fn: base},
			"clean":         &object.Function{Name: "clean", Fn: clean},
			"dir":           &object.Function{Name: "dir", Fn: dir},
			"ext":           &object.Function{Name: "ext", Fn: ext},
			"from_slash":    &object.Function{Name: "from_slash", Fn: fromSlash},
			"to_slash":      &object.Function{Name: "to_slash", Fn: toSlash},
			"is_abs":        &object.Function{Name: "is_abs", Fn: isAbs},
			"join":          &object.Function{Name: "join", Fn: join},
			"match":         &object.Function{Name: "match", Fn: match},
			"split":         &object.Function{Name: "split", Fn: split},
			"split_list":    &object.Function{Name: "split_list", Fn: splitList},
			"rel":           &object.Function{Name: "rel", Fn: rel},
			"volume_name":   &object.Function{Name: "volume_name", Fn: volumeName},
			"glob":          &object.Function{Name: "glob", Fn: glob},
			"eval_symlinks": &object.Function{Name: "eval_symlinks", Fn: evalSymlinks},
		},
	}, nil
}

func getStringArg(fnName string, args object.CallArgs, expected int, idx int) (string, error) {
	if len(args.Positional) != expected {
		return "", fmt.Errorf("%s() takes exactly %d argument(s), got %d", fnName, expected, len(args.Positional))
	}
	s, ok := args.Positional[idx].(object.String)
	if !ok {
		return "", fmt.Errorf("%s() argument must be a string", fnName)
	}
	return string(s), nil
}

func abs(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("abs", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("abs", args, 1, 0)
	if err != nil {
		return nil, err
	}
	absPath, err := stdpath.Abs(path)
	if err != nil {
		return nil, err
	}
	return object.String(absPath), nil
}

func base(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("base", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("base", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.Base(path)), nil
}

func clean(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("clean", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("clean", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.Clean(path)), nil
}

func dir(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("dir", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("dir", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.Dir(path)), nil
}

func ext(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("ext", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("ext", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.Ext(path)), nil
}

func fromSlash(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("from_slash", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("from_slash", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.FromSlash(path)), nil
}

func toSlash(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("to_slash", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("to_slash", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.ToSlash(path)), nil
}

func isAbs(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("is_abs", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("is_abs", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.Bool(stdpath.IsAbs(path)), nil
}


func join(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("join", args); err != nil {
		return nil, err
	}
	elems := make([]string, len(args.Positional))
	for i, arg := range args.Positional {
		s, ok := arg.(object.String)
		if !ok {
			return nil, fmt.Errorf("join() argument %d must be a string", i)
		}
		elems[i] = string(s)
	}
	return object.String(stdpath.Join(elems...)), nil
}

func match(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("match", args); err != nil {
		return nil, err
	}
	pattern, err := getStringArg("match", args, 2, 0)
	if err != nil {
		return nil, err
	}
	name, ok := args.Positional[1].(object.String)
	if !ok {
		return nil, fmt.Errorf("match() second argument must be a string")
	}
	matched, err := stdpath.Match(pattern, string(name))
	if err != nil {
		return nil, err
	}
	return object.Bool(matched), nil
}

func split(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("split", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("split", args, 1, 0)
	if err != nil {
		return nil, err
	}
	d, f := stdpath.Split(path)
	return &object.List{Elements: []object.Object{object.String(d), object.String(f)}}, nil
}

func splitList(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("split_list", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("split_list", args, 1, 0)
	if err != nil {
		return nil, err
	}
	parts := stdpath.SplitList(path)
	elements := make([]object.Object, len(parts))
	for i, p := range parts {
		elements[i] = object.String(p)
	}
	return &object.List{Elements: elements}, nil
}

func rel(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("rel", args); err != nil {
		return nil, err
	}
	basepath, err := getStringArg("rel", args, 2, 0)
	if err != nil {
		return nil, err
	}
	targpath, ok := args.Positional[1].(object.String)
	if !ok {
		return nil, fmt.Errorf("rel() second argument must be a string")
	}
	relPath, err := stdpath.Rel(basepath, string(targpath))
	if err != nil {
		return nil, err
	}
	return object.String(relPath), nil
}

func volumeName(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("volume_name", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("volume_name", args, 1, 0)
	if err != nil {
		return nil, err
	}
	return object.String(stdpath.VolumeName(path)), nil
}

func glob(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("glob", args); err != nil {
		return nil, err
	}
	pattern, err := getStringArg("glob", args, 1, 0)
	if err != nil {
		return nil, err
	}
	matches, err := stdpath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	elements := make([]object.Object, len(matches))
	for i, m := range matches {
		elements[i] = object.String(m)
	}
	return &object.List{Elements: elements}, nil
}

func evalSymlinks(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("eval_symlinks", args); err != nil {
		return nil, err
	}
	path, err := getStringArg("eval_symlinks", args, 1, 0)
	if err != nil {
		return nil, err
	}
	resolved, err := stdpath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	return object.String(resolved), nil
}
