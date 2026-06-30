package extension

import (
	"fmt"
	"os"

	"github.com/aisk/goblin/object"
)

func ExecuteOs() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"exit":         &object.Function{Name: "exit", Fn: exit},
			"getegid":      &object.Function{Name: "getegid", Fn: getegid},
			"getenv":       &object.Function{Name: "getenv", Fn: getenv},
			"geteuid":      &object.Function{Name: "geteuid", Fn: geteuid},
			"getgid":       &object.Function{Name: "getgid", Fn: getgid},
			"getgroups":    &object.Function{Name: "getgroups", Fn: getgroups},
			"getpagesize":  &object.Function{Name: "getpagesize", Fn: getpagesize},
			"getpid":       &object.Function{Name: "getpid", Fn: getpid},
			"getppid":      &object.Function{Name: "getppid", Fn: getppid},
			"getuid":       &object.Function{Name: "getuid", Fn: getuid},
			"getwd":        &object.Function{Name: "getwd", Fn: getwd},
			"tempdir":  &object.Function{Name: "tempdir", Fn: tempDir},
			"tempfile": &object.Function{Name: "tempfile", Fn: tempFile},
		},
	}, nil
}

func exit(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("exit", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("exit() requires exactly 1 argument")
	}
	code, ok := args.Positional[0].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("exit() argument must be an integer")
	}
	os.Exit(int(code))
	return nil, nil
}

func getenv(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getenv", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, fmt.Errorf("getenv() requires exactly 1 argument")
	}
	key, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, fmt.Errorf("getenv() argument must be a string")
	}
	value := os.Getenv(string(key))
	return object.String(value), nil
}

func getpid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getpid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getpid() requires no arguments")
	}
	pid := os.Getpid()
	return object.Integer(pid), nil
}

func getppid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getppid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getppid() requires no arguments")
	}
	ppid := os.Getppid()
	return object.Integer(ppid), nil
}

func getuid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getuid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getuid() requires no arguments")
	}
	uid := os.Getuid()
	return object.Integer(uid), nil
}

func getegid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getegid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getegid() requires no arguments")
	}
	egid := os.Getegid()
	return object.Integer(egid), nil
}

func geteuid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("geteuid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("geteuid() requires no arguments")
	}
	euid := os.Geteuid()
	return object.Integer(euid), nil
}

func getgid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getgid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getgid() requires no arguments")
	}
	gid := os.Getgid()
	return object.Integer(gid), nil
}

func getgroups(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getgroups", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getgroups() requires no arguments")
	}
	gids, err := os.Getgroups()
	if err != nil {
		return nil, err
	}
	elems := make([]object.Object, len(gids))
	for i, g := range gids {
		elems[i] = object.Integer(g)
	}
	return &object.List{Elements: elems}, nil
}

func getpagesize(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getpagesize", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getpagesize() requires no arguments")
	}
	return object.Integer(os.Getpagesize()), nil
}

func getwd(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getwd", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, fmt.Errorf("getwd() requires no arguments")
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return object.String(wd), nil
}

// tempDir creates a new temporary directory, mirroring Go's os.MkdirTemp.
// Both dir and pattern are optional; when dir is empty Go uses the OS default
// temp directory.
func tempDir(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("tempdir", args); err != nil {
		return nil, err
	}
	if len(args.Positional) > 2 {
		return nil, fmt.Errorf("tempdir() takes at most 2 arguments, got %d", len(args.Positional))
	}
	dir := ""
	pattern := ""
	if len(args.Positional) >= 1 {
		d, ok := args.Positional[0].(object.String)
		if !ok {
			return nil, fmt.Errorf("tempdir() first argument (dir) must be a string")
		}
		dir = string(d)
	}
	if len(args.Positional) >= 2 {
		p, ok := args.Positional[1].(object.String)
		if !ok {
			return nil, fmt.Errorf("tempdir() second argument (pattern) must be a string")
		}
		pattern = string(p)
	}
	path, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return object.String(path), nil
}

// tempFile creates a new temporary file, mirroring Go's os.CreateTemp.
// Both dir and pattern are optional; when dir is empty Go uses the OS default
// temp directory.
func tempFile(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("tempfile", args); err != nil {
		return nil, err
	}
	if len(args.Positional) > 2 {
		return nil, fmt.Errorf("tempfile() takes at most 2 arguments, got %d", len(args.Positional))
	}
	dir := ""
	pattern := ""
	if len(args.Positional) >= 1 {
		d, ok := args.Positional[0].(object.String)
		if !ok {
			return nil, fmt.Errorf("tempfile() first argument (dir) must be a string")
		}
		dir = string(d)
	}
	if len(args.Positional) >= 2 {
		p, ok := args.Positional[1].(object.String)
		if !ok {
			return nil, fmt.Errorf("tempfile() second argument (pattern) must be a string")
		}
		pattern = string(p)
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	f.Close()
	return object.String(f.Name()), nil
}
