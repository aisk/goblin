// Package exec is the exec module: run other programs. A command is a List,
// the program followed by its arguments; no shell is involved.
package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	stdfs "io/fs"
	"os"
	stdexec "os/exec"
	"strings"
	"time"

	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

var (
	commandType = object.NewOpaqueConstructor("Command", "exec.start(args)")
	resultType  = object.NewOpaqueConstructor("Result", "exec.run(args)")
	pipeType    = object.NewOpaqueConstructor("Pipe", "exec.PIPE")
)

// Pipe is the type of exec.PIPE, which asks exec.start to connect a
// standard stream of the command to this program.
type Pipe struct{ object.OpaqueBase }

var pipe = &Pipe{OpaqueBase: object.MakeOpaqueBase("Pipe")}

func (p *Pipe) Constructor() *object.Function            { return pipeType.Function }
func (p *Pipe) String() string                           { return "exec.PIPE" }
func (p *Pipe) ToString() (string, error)                { return p.String(), nil }
func (p *Pipe) Equals(other object.Object) (bool, error) { return other == object.Object(p), nil }
func (p *Pipe) Attributes() []string                     { return nil }
func (p *Pipe) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Pipe has no attribute '%s'", name)
}

// ExitError is raised by run and capture when a command exits with a
// non-zero status under check=true. Data: code, stdout, stderr.
var ExitError = object.NewErrorKind("ExitError", object.BaseError)

func Execute() (object.Object, error) {
	return object.NewModule("exec", map[string]object.Object{
		"ExitError": ExitError,
		"PIPE":      pipe,
		"Command":   commandType.Function,
		"Result":    resultType.Function,
		"Pipe":      pipeType.Function,
	}, object.Funcs{
		"run":     run,
		"capture": capture,
		"start":   start,
		"find":    find,
	}), nil
}

// spec is the parsed form of the parameters run, capture and start share.
type spec struct {
	fn      string
	argv    []string
	cwd     string
	hasCwd  bool
	env     *object.Dict
	stdin   object.Object
	stdout  object.Object
	stderr  object.Object
	check   bool
	timeout time.Duration
	limited bool
}

// parse binds the parameters of fn in their documented order; output and
// check/timeout are only bound for the functions that take them.
func parse(fn string, args object.CallArgs, output, wait bool) (*spec, error) {
	p := object.NewArgParser(fn, args)
	s := &spec{fn: fn}
	list := p.List("args")
	s.cwd, s.hasCwd = p.NullablePath("cwd")
	envObj := p.AnyOr("env", object.Nil)
	s.stdin = p.AnyOr("stdin", object.Nil)
	s.stdout, s.stderr = object.Nil, object.Nil
	if output {
		s.stdout = p.AnyOr("stdout", object.Nil)
		s.stderr = p.AnyOr("stderr", object.Nil)
	}
	s.check = true
	if wait {
		s.check = bool(p.BoolOr("check", true))
		s.timeout, s.limited = p.Timeout("timeout")
	}
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if len(list.Elements) == 0 {
		return nil, object.NewValueError("%s() argument 'args' must not be empty", fn)
	}
	s.argv = make([]string, len(list.Elements))
	for i, arg := range list.Elements {
		text, ok := object.PathString(arg)
		if !ok {
			return nil, object.NewTypeError("%s() argument 'args' must contain String or Path, got %s at index %d", fn, arg.TypeName(), i)
		}
		s.argv[i] = text
	}
	if envObj != object.Nil {
		dict, ok := envObj.(*object.Dict)
		if !ok {
			return nil, object.NewTypeError("%s() argument 'env' must be Dict or Nil, got %s", fn, envObj.TypeName())
		}
		for _, entry := range dict.Entries() {
			if _, ok := entry.Key.(object.String); !ok {
				return nil, object.NewTypeError("%s() argument 'env' keys must be String, got %s", fn, entry.Key.TypeName())
			}
			if _, ok := entry.Value.(object.String); !ok && entry.Value != object.Nil {
				return nil, object.NewTypeError("%s() argument 'env' values must be String or Nil, got %s", fn, entry.Value.TypeName())
			}
		}
		s.env = dict
	}
	return s, nil
}

// environ is this process's environment with env applied in insertion
// order: a String sets a variable, nil removes it.
func environ(env *object.Dict) []string {
	vars := os.Environ()
	if env == nil {
		return vars
	}
	for _, entry := range env.Entries() {
		name := string(entry.Key.(object.String))
		kept := vars[:0]
		for _, v := range vars {
			if n, _, _ := strings.Cut(v, "="); n != name {
				kept = append(kept, v)
			}
		}
		vars = kept
		if value, ok := entry.Value.(object.String); ok {
			vars = append(vars, name+"="+string(value))
		}
	}
	return vars
}

// command builds the Go command for s. ctx bounds it when a timeout is set.
func (s *spec) command(ctx context.Context) (*stdexec.Cmd, error) {
	var cmd *stdexec.Cmd
	if ctx != nil {
		cmd = stdexec.CommandContext(ctx, s.argv[0], s.argv[1:]...)
		// A killed command's output pipes may be held open by its own
		// children; stop waiting for them shortly after the kill.
		cmd.WaitDelay = time.Second
	} else {
		cmd = stdexec.Command(s.argv[0], s.argv[1:]...)
	}
	if s.hasCwd {
		info, err := os.Stat(s.cwd)
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, s.fn+`() working directory "`+s.cwd+`"`, unwrapPath(err)).
				With("path", pathvalue.NewPath(s.cwd))
		}
		if !info.IsDir() {
			return nil, object.NewKindError(object.NotExistError, `%s() working directory "%s" is not a directory`, s.fn, s.cwd).
				With("path", pathvalue.NewPath(s.cwd))
		}
		cmd.Dir = s.cwd
	}
	cmd.Env = environ(s.env)
	return cmd, nil
}

// unwrapPath strips the operation, path or program name Go adds to an
// error, since the message wrapping it already names them.
func unwrapPath(err error) error {
	var pe *stdfs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var ee *stdexec.Error
	if errors.As(err, &ee) {
		return ee.Err
	}
	return err
}

// stdinReader is the command's stdin for a data value: nil inherits this
// process's stdin.
func (s *spec) stdinReader(allowPipe bool) (io.Reader, error) {
	if s.stdin == object.Nil {
		return os.Stdin, nil
	}
	if s.stdin == object.Object(pipe) && !allowPipe {
		return nil, object.NewTypeError("%s() argument 'stdin' does not accept exec.PIPE (use exec.start)", s.fn)
	}
	r, ok := object.DataReader(s.fn, "stdin", s.stdin)
	if !ok {
		want := "String, Bytes, Nil or have a read(size) method"
		if allowPipe {
			want = "String, Bytes, Nil, exec.PIPE or have a read(size) method"
		}
		return nil, object.NewTypeError("%s() argument 'stdin' must be %s, got %s", s.fn, want, s.stdin.TypeName())
	}
	return r, nil
}

// outputWriter is the destination of a command's stdout or stderr: nil
// inherits this process's stream; a writable receives the output and is not
// closed.
func (s *spec) outputWriter(name string, value object.Object, inherit *os.File, allowPipe bool) (io.Writer, error) {
	if value == object.Nil {
		return inherit, nil
	}
	if value == object.Object(pipe) && !allowPipe {
		return nil, object.NewTypeError("%s() argument '%s' does not accept exec.PIPE (use exec.start)", s.fn, name)
	}
	w, err := object.NewDuckWriter(s.fn, name, value)
	if err != nil {
		want := "Nil or have a write(data) method"
		if allowPipe {
			want = "Nil, exec.PIPE or have a write(data) method"
		}
		return nil, object.NewTypeError("%s() argument '%s' must be %s, got %s", s.fn, name, want, value.TypeName())
	}
	return writeOnly{w}, nil
}

// writeOnly hides the Close of a DuckWriter: exec never closes a stream it
// was handed.
type writeOnly struct{ w io.Writer }

func (w writeOnly) Write(p []byte) (int, error) { return w.w.Write(p) }

// startError reports a command that could not be started.
func (s *spec) startError(err error) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) {
		return err
	}
	cause := unwrapPath(err)
	if errors.Is(err, stdexec.ErrNotFound) || errors.Is(err, stdfs.ErrNotExist) {
		return object.WrapError(object.NotExistError, s.fn+`() program "`+s.argv[0]+`" not found`, cause)
	}
	return object.WrapNativeError(object.IOError, s.fn+`() cannot run "`+s.argv[0]+`"`, cause)
}

func run(args object.CallArgs) (object.Object, error) {
	s, err := parse("run", args, true, true)
	if err != nil {
		return nil, err
	}
	stdout, err := s.outputWriter("stdout", s.stdout, os.Stdout, false)
	if err != nil {
		return nil, err
	}
	stderr, err := s.outputWriter("stderr", s.stderr, os.Stderr, false)
	if err != nil {
		return nil, err
	}
	// One writable passed for both streams is written by one goroutine at
	// a time, as os/exec does for a shared writer.
	if s.stdout != object.Nil && s.stdout == s.stderr {
		stderr = stdout
	}
	return s.complete(stdout, stderr, nil, nil)
}

func capture(args object.CallArgs) (object.Object, error) {
	s, err := parse("capture", args, false, true)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	return s.complete(&stdout, &stderr, &stdout, &stderr)
}

// complete runs s to completion and builds its Result; captured buffers,
// when given, become the Result's text.
func (s *spec) complete(stdout, stderr io.Writer, outBuf, errBuf *bytes.Buffer) (object.Object, error) {
	stdin, err := s.stdinReader(false)
	if err != nil {
		return nil, err
	}
	var ctx context.Context
	if s.limited {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
	}
	cmd, err := s.command(ctx)
	if err != nil {
		return nil, err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Start(); err != nil {
		return nil, s.startError(err)
	}
	err = cmd.Wait()
	if ctx != nil && ctx.Err() != nil {
		return nil, object.NewTimeoutError(`%s() "%s" timed out after %g seconds`, s.fn, s.argv[0], s.timeout.Seconds()).
			With("stdout", text(outBuf)).
			With("stderr", text(errBuf))
	}
	if err != nil {
		var exitErr *stdexec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, s.startError(err)
		}
	}
	result := newResult(cmd.ProcessState.ExitCode(), outBuf, errBuf)
	if s.check && result.code != 0 {
		message := s.fn + `() "` + s.argv[0] + `" exited with status ` + itoa(result.code)
		if errBuf != nil {
			if line := lastLine(errBuf.String()); line != "" {
				message += ": " + line
			}
		}
		return nil, object.NewKindError(ExitError, "%s", message).
			With("code", object.Integer(result.code)).
			With("stdout", result.stdout).
			With("stderr", result.stderr)
	}
	return result, nil
}

func lastLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

func find(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("find", args)
	name := p.Str("name")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	path, err := stdexec.LookPath(string(name))
	if err != nil || name == "" {
		return object.Nil, nil
	}
	return pathvalue.NewPath(path), nil
}
