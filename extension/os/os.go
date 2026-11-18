// Package os is the os module: the running process's arguments,
// environment, standard streams and exit status. Files are fs and other
// programs are exec.
package os

import (
	"os"
	"runtime"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/procargs"
	"github.com/aisk/goblin/object"
)

// The standard streams are shared by every load of the module, so a line
// buffer on stdin is never split between two values; stdin reads through
// object.Stdin, the buffer input() uses too. Each one looks up the process's
// stream when it is used, as print does, so a program that redirects
// os.Stdout (a test capturing output) redirects both alike.
var (
	stdin  = filestream.NewReader("stdin", object.Stdin, stdStream{func() *os.File { return os.Stdin }})
	stdout = filestream.NewWriter("stdout", stdStream{func() *os.File { return os.Stdout }}, stdStream{func() *os.File { return os.Stdout }})
	stderr = filestream.NewWriter("stderr", stdStream{func() *os.File { return os.Stderr }}, stdStream{func() *os.File { return os.Stderr }})
)

// stdStream is one of the process's standard streams, looked up on use.
type stdStream struct{ file func() *os.File }

func (s stdStream) Read(p []byte) (int, error)  { return s.file().Read(p) }
func (s stdStream) Write(p []byte) (int, error) { return s.file().Write(p) }
func (s stdStream) Close() error                { return s.file().Close() }

// Execute builds the os module from the process's own command line. Used by
// compiled programs, where os.Args[0] is the executable.
func Execute() (object.Object, error) {
	return newModule(procargs.Process()), nil
}

// ExecuteWithFrozenArgs builds the os module from a given command line, the
// script path followed by its arguments. The interpreter passes the script's
// argv, the REPL [""], so a run never sees the goblin binary's own arguments.
func ExecuteWithFrozenArgs(argv []string) (object.Object, error) {
	return newModule(argv), nil
}

func newModule(argv []string) object.Object {
	program, rest := procargs.Split(argv)
	args := make([]object.Object, len(rest))
	for i, a := range rest {
		args[i] = object.String(a)
	}
	return object.NewModule("os", map[string]object.Object{
		"args":     &object.List{Elements: args},
		"program":  object.String(program),
		"env":      theEnv,
		"Env":      envType.Function,
		"stdin":    stdin,
		"stdout":   stdout,
		"stderr":   stderr,
		"pid":      object.Integer(os.Getpid()),
		"platform": object.String(runtime.GOOS),
	}, object.Funcs{
		"exit": exit,
	})
}

// exit implements os.exit(code=0): it ends the program with code after the
// calling goblin has run its pending deferred calls. No catch clause
// receives it.
func exit(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("exit", args)
	code := p.IntOr("code", 0)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if code < 0 || code > 255 {
		return nil, object.NewValueError("exit() argument 'code' must be between 0 and 255, got %d", int64(code))
	}
	// The request unwinds the calling goblin like an error, running its
	// pending deferred calls, and the process ends at the top of the goblin
	// (object.NewExit).
	return nil, object.NewExit(int(code))
}
