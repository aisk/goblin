package extension

import (
	"os"
	"strings"

	"github.com/aisk/goblin/extension/fs"
	"github.com/aisk/goblin/object"
)

// ExecuteOs builds the os module with live process arguments (os.Args).
// Used by compiled programs from build-exe.
func ExecuteOs() (object.Object, error) {
	return newOsModule(func() []string { return os.Args }), nil
}

// ExecuteOsWithFrozenArgs builds the os module with a fixed Args snapshot.
// Used by the interpreter so goblin run / the REPL expose script argv without
// mutating process-global state. The snapshot is closed over by argv(), so
// concurrent runs and spawned goroutines keep a stable view after the loader
// returns.
func ExecuteOsWithFrozenArgs(args []string) (object.Object, error) {
	snapshot := append([]string(nil), args...)
	return newOsModule(func() []string { return snapshot }), nil
}

func newOsModule(argsFn func() []string) object.Object {
	return &object.Module{
		Members: map[string]object.Object{
			"argv":        &object.Function{Name: "argv", Fn: makeArgv(argsFn)},
			"environ":     &object.Function{Name: "environ", Fn: environ},
			"exit":        &object.Function{Name: "exit", Fn: exit},
			"getegid":     &object.Function{Name: "getegid", Fn: getegid},
			"getenv":      &object.Function{Name: "getenv", Fn: getenv},
			"geteuid":     &object.Function{Name: "geteuid", Fn: geteuid},
			"getgid":      &object.Function{Name: "getgid", Fn: getgid},
			"getgroups":   &object.Function{Name: "getgroups", Fn: getgroups},
			"getpagesize": &object.Function{Name: "getpagesize", Fn: getpagesize},
			"getpid":      &object.Function{Name: "getpid", Fn: getpid},
			"getppid":     &object.Function{Name: "getppid", Fn: getppid},
			"getuid":      &object.Function{Name: "getuid", Fn: getuid},
			"getwd":       &object.Function{Name: "getwd", Fn: getwd},
			"hostname":    &object.Function{Name: "hostname", Fn: hostname},
			"setenv":      &object.Function{Name: "setenv", Fn: setenv},
			"unsetenv":    &object.Function{Name: "unsetenv", Fn: unsetenv},
			"mkdir_temp":  &object.Function{Name: "mkdir_temp", Fn: mkdirTemp},
			"create_temp": &object.Function{Name: "create_temp", Fn: createTemp},
		},
	}
}

// makeArgv returns argv(), which yields a fresh Goblin list from argsFn each call.
func makeArgv(argsFn func() []string) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		if err := object.RequireNoKeyword("argv", args); err != nil {
			return nil, err
		}
		if len(args.Positional) != 0 {
			return nil, object.NewTypeError("argv() requires no arguments")
		}
		procArgs := argsFn()
		elems := make([]object.Object, len(procArgs))
		for i, a := range procArgs {
			elems[i] = object.String(a)
		}
		return &object.List{Elements: elems}, nil
	}
}

func exit(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("exit", args); err != nil {
		return nil, err
	}
	if len(args.Positional) > 1 {
		return nil, object.NewTypeError("exit() takes at most 1 argument")
	}
	code := 0
	if len(args.Positional) == 1 {
		c, ok := args.Positional[0].(object.Integer)
		if !ok {
			return nil, object.NewTypeError("exit() argument must be an integer")
		}
		code = int(c)
	}
	os.Exit(code)
	return nil, nil
}

func getenv(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getenv", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, object.NewTypeError("getenv() requires exactly 1 argument")
	}
	key, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, object.NewTypeError("getenv() argument must be a string")
	}
	value := os.Getenv(string(key))
	return object.String(value), nil
}

func setenv(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("setenv", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 2 {
		return nil, object.NewTypeError("setenv() requires exactly 2 arguments")
	}
	key, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, object.NewTypeError("setenv() first argument must be a string")
	}
	value, ok := args.Positional[1].(object.String)
	if !ok {
		return nil, object.NewTypeError("setenv() second argument must be a string")
	}
	if err := os.Setenv(string(key), string(value)); err != nil {
		return nil, object.WrapNativeError(object.IOError, "setenv() failed", err)
	}
	return object.Nil, nil
}

func unsetenv(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("unsetenv", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 1 {
		return nil, object.NewTypeError("unsetenv() requires exactly 1 argument")
	}
	key, ok := args.Positional[0].(object.String)
	if !ok {
		return nil, object.NewTypeError("unsetenv() argument must be a string")
	}
	if err := os.Unsetenv(string(key)); err != nil {
		return nil, object.WrapNativeError(object.IOError, "unsetenv() failed", err)
	}
	return object.Nil, nil
}

func environ(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("environ", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("environ() requires no arguments")
	}
	env := os.Environ()
	entries := make(map[string]object.DictEntry, len(env))
	for _, e := range env {
		key, value, found := strings.Cut(e, "=")
		if !found {
			continue
		}
		entries[key] = object.DictEntry{
			Key:   object.String(key),
			Value: object.String(value),
		}
	}
	return &object.Dict{Entries: entries}, nil
}

func hostname(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("hostname", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("hostname() requires no arguments")
	}
	name, err := os.Hostname()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "hostname() failed", err)
	}
	return object.String(name), nil
}

func getpid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getpid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getpid() requires no arguments")
	}
	pid := os.Getpid()
	return object.Integer(pid), nil
}

func getppid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getppid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getppid() requires no arguments")
	}
	ppid := os.Getppid()
	return object.Integer(ppid), nil
}

func getuid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getuid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getuid() requires no arguments")
	}
	uid := os.Getuid()
	return object.Integer(uid), nil
}

func getegid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getegid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getegid() requires no arguments")
	}
	egid := os.Getegid()
	return object.Integer(egid), nil
}

func geteuid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("geteuid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("geteuid() requires no arguments")
	}
	euid := os.Geteuid()
	return object.Integer(euid), nil
}

func getgid(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getgid", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getgid() requires no arguments")
	}
	gid := os.Getgid()
	return object.Integer(gid), nil
}

func getgroups(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getgroups", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getgroups() requires no arguments")
	}
	gids, err := os.Getgroups()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "getgroups() failed", err)
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
		return nil, object.NewTypeError("getpagesize() requires no arguments")
	}
	return object.Integer(os.Getpagesize()), nil
}

func getwd(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("getwd", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("getwd() requires no arguments")
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "getwd() failed", err)
	}
	return object.String(wd), nil
}

// tempDir creates a new temporary directory, mirroring Go's os.MkdirTemp.
// Both dir and pattern are optional; when dir is empty Go uses the OS default
// temp directory.
func mkdirTemp(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("mkdir_temp", args)
	dirObj := ap.AnyOr("dir", object.String(""))
	pattern := ap.StrOr("pattern", "")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	dir, ok := object.PathString(dirObj)
	if !ok {
		return nil, object.NewTypeError("mkdir_temp() argument 'dir' must be a string or Path")
	}
	path, err := os.MkdirTemp(dir, string(pattern))
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "mkdir_temp() failed", err)
	}
	return object.String(path), nil
}

// tempFile creates a new temporary file, mirroring Go's os.CreateTemp.
// Both dir and pattern are optional; when dir is empty Go uses the OS default
// temp directory.
func createTemp(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("create_temp", args)
	dirObj := ap.AnyOr("dir", object.String(""))
	pattern := ap.StrOr("pattern", "")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	dir, ok := object.PathString(dirObj)
	if !ok {
		return nil, object.NewTypeError("create_temp() argument 'dir' must be a string or Path")
	}
	file, err := os.CreateTemp(dir, string(pattern))
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "create_temp() failed", err)
	}
	return fs.NewFile(file.Name(), file), nil
}
