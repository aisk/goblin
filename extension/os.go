package extension

import (
	"os"
	"strings"

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
		Name: "os",
		Members: map[string]object.Object{
			"argv":        &object.Function{Name: "argv", Fn: makeArgv(argsFn)},
			"environ":     &object.Function{Name: "environ", Fn: environ},
			"exit":        &object.Function{Name: "exit", Fn: exit},
			"getegid":     intGetter("getegid", os.Getegid),
			"getenv":      &object.Function{Name: "getenv", Fn: getenv},
			"geteuid":     intGetter("geteuid", os.Geteuid),
			"getgid":      intGetter("getgid", os.Getgid),
			"getgroups":   &object.Function{Name: "getgroups", Fn: getgroups},
			"getpagesize": &object.Function{Name: "getpagesize", Fn: getpagesize},
			"getpid":      intGetter("getpid", os.Getpid),
			"getppid":     intGetter("getppid", os.Getppid),
			"getuid":      intGetter("getuid", os.Getuid),
			"getwd":       &object.Function{Name: "getwd", Fn: getwd},
			"hostname":    &object.Function{Name: "hostname", Fn: hostname},
			"setenv":      &object.Function{Name: "setenv", Fn: setenv},
			"unsetenv":    &object.Function{Name: "unsetenv", Fn: unsetenv},
			"tempdir":     &object.Function{Name: "tempdir", Fn: tempDir},
			"tempfile":    &object.Function{Name: "tempfile", Fn: tempFile},
		},
	}
}

// makeArgv returns argv(), which yields a fresh Goblin list from argsFn each call.
func makeArgv(argsFn func() []string) func(object.CallArgs) (object.Object, error) {
	return func(args object.CallArgs) (object.Object, error) {
		if err := object.RequireNoArgs("argv", args); err != nil {
			return nil, err
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
	p := object.NewArgParser("exit", args)
	code := p.IntOr("code", 0)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	os.Exit(int(code))
	return nil, nil
}

func getenv(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("getenv", args)
	key := p.Str("key")
	def := p.AnyOr("default", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	value, ok := os.LookupEnv(string(key))
	if !ok {
		return def, nil
	}
	return object.String(value), nil
}

func setenv(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("setenv", args)
	key, value := p.Str("key"), p.Str("value")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if err := os.Setenv(string(key), string(value)); err != nil {
		return nil, object.WrapNativeError(object.IOError, "setenv() failed", err)
	}
	return object.Nil, nil
}

func unsetenv(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("unsetenv", args)
	key := p.Str("key")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if err := os.Unsetenv(string(key)); err != nil {
		return nil, object.WrapNativeError(object.IOError, "unsetenv() failed", err)
	}
	return object.Nil, nil
}

func environ(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("environ", args); err != nil {
		return nil, err
	}
	result := object.NewDict()
	for _, e := range os.Environ() {
		key, value, found := strings.Cut(e, "=")
		if !found {
			continue
		}
		if err := result.Set(object.String(key), object.String(value)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// intGetter wraps a nullary os query returning an int as a builtin.
func intGetter(name string, fn func() int) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		if err := object.RequireNoArgs(name, args); err != nil {
			return nil, err
		}
		return object.Integer(fn()), nil
	}}
}

func hostname(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("hostname", args); err != nil {
		return nil, err
	}
	name, err := os.Hostname()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "hostname() failed", err)
	}
	return object.String(name), nil
}

func getgroups(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("getgroups", args); err != nil {
		return nil, err
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
	if err := object.RequireNoArgs("getpagesize", args); err != nil {
		return nil, err
	}
	return object.Integer(os.Getpagesize()), nil
}

func getwd(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("getwd", args); err != nil {
		return nil, err
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
func tempDir(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("tempdir", args)
	dirObj, hasDir := p.OptionalAny("dir")
	pattern := p.StrOr("pattern", "")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	dir := ""
	if hasDir {
		d, ok := object.PathString(dirObj)
		if !ok {
			return nil, object.NewTypeError("tempdir() first argument (dir) must be a string or Path")
		}
		dir = d
	}
	path, err := os.MkdirTemp(dir, string(pattern))
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "tempdir() failed", err)
	}
	return object.String(path), nil
}

// tempFile creates a new temporary file, mirroring Go's os.CreateTemp.
// Both dir and pattern are optional; when dir is empty Go uses the OS default
// temp directory.
func tempFile(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("tempfile", args)
	dirObj, hasDir := p.OptionalAny("dir")
	pattern := p.StrOr("pattern", "")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	dir := ""
	if hasDir {
		d, ok := object.PathString(dirObj)
		if !ok {
			return nil, object.NewTypeError("tempfile() first argument (dir) must be a string or Path")
		}
		dir = d
	}
	f, err := os.CreateTemp(dir, string(pattern))
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "tempfile() failed", err)
	}
	f.Close()
	return object.String(f.Name()), nil
}
