package path

import (
	stdpath "path"

	"github.com/aisk/goblin/object"
)

// Execute returns the path module, exposing Go's path functions.
// Unlike filepath, path always uses forward slashes regardless of OS.
func Execute() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"base":    &object.Function{Name: "base", Fn: base},
			"clean":   &object.Function{Name: "clean", Fn: clean},
			"dir":     &object.Function{Name: "dir", Fn: dir},
			"ext":     &object.Function{Name: "ext", Fn: ext},
			"is_abs":  &object.Function{Name: "is_abs", Fn: isAbs},
			"join":    &object.Function{Name: "join", Fn: join},
			"match":   &object.Function{Name: "match", Fn: match},
			"split":   &object.Function{Name: "split", Fn: split},
		},
	}, nil
}

func getStringArg(fnName string, args object.CallArgs, expected int, idx int) (string, error) {
	if len(args.Positional) != expected {
		return "", object.NewTypeError("%s() takes exactly %d argument(s), got %d", fnName, expected, len(args.Positional))
	}
	s, ok := args.Positional[idx].(object.String)
	if !ok {
		return "", object.NewTypeError("%s() argument must be a string", fnName)
	}
	return string(s), nil
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
			return nil, object.NewTypeError("join() argument %d must be a string", i)
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
		return nil, object.NewTypeError("match() second argument must be a string")
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
