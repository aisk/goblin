package exec

import (
	"errors"
	"os"
	stdexec "os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

// Command is a started process, from exec.start.
type Command struct {
	object.OpaqueBase
	argv []string
	cmd  *stdexec.Cmd
	// stdin, stdout and stderr are the pipes asked for with exec.PIPE, or
	// nil.
	stdin, stdout, stderr object.Object
	// done is closed once the process has been waited for; result and
	// waitErr are set before.
	done    chan struct{}
	mu      sync.Mutex
	result  *Result
	waitErr error
}

func start(args object.CallArgs) (object.Object, error) {
	s, err := parse("start", args, true, false)
	if err != nil {
		return nil, err
	}
	cmd, err := s.command(nil)
	if err != nil {
		return nil, err
	}
	c := &Command{OpaqueBase: object.MakeOpaqueBase("Command"), argv: s.argv, cmd: cmd, stdin: object.Nil, stdout: object.Nil, stderr: object.Nil}
	// The child's ends of the pipes, closed here once it has started.
	var childEnds []*os.File
	closeAll := func(files []*os.File) {
		for _, f := range files {
			f.Close()
		}
	}
	var ours []*os.File
	fail := func(err error) (object.Object, error) {
		closeAll(childEnds)
		closeAll(ours)
		return nil, err
	}

	if s.stdin == object.Object(pipe) {
		r, w, err := os.Pipe()
		if err != nil {
			return fail(object.WrapNativeError(object.IOError, "start() cannot create a pipe", err))
		}
		childEnds, ours = append(childEnds, r), append(ours, w)
		cmd.Stdin = r
		c.stdin = filestream.NewWriter("stdin pipe", w, w)
	} else {
		stdin, err := s.stdinReader(true)
		if err != nil {
			return fail(err)
		}
		cmd.Stdin = stdin
	}
	for _, stream := range []struct {
		name    string
		value   object.Object
		inherit *os.File
		set     func(f *os.File)
		field   *object.Object
	}{
		{"stdout", s.stdout, os.Stdout, func(f *os.File) { cmd.Stdout = f }, &c.stdout},
		{"stderr", s.stderr, os.Stderr, func(f *os.File) { cmd.Stderr = f }, &c.stderr},
	} {
		if stream.value == object.Object(pipe) {
			r, w, err := os.Pipe()
			if err != nil {
				return fail(object.WrapNativeError(object.IOError, "start() cannot create a pipe", err))
			}
			childEnds, ours = append(childEnds, w), append(ours, r)
			stream.set(w)
			*stream.field = filestream.NewReader(stream.name+" pipe", r, r)
			continue
		}
		w, err := s.outputWriter(stream.name, stream.value, stream.inherit, true)
		if err != nil {
			return fail(err)
		}
		if stream.name == "stdout" {
			cmd.Stdout = w
		} else if s.stderr == s.stdout && s.stdout != object.Nil {
			cmd.Stderr = cmd.Stdout
		} else {
			cmd.Stderr = w
		}
	}
	if err := cmd.Start(); err != nil {
		return fail(s.startError(err))
	}
	closeAll(childEnds)
	c.done = make(chan struct{})
	go c.reap()
	return c, nil
}

// reap waits for the process in the background, so running is accurate and
// wait can give up after a timeout.
func (c *Command) reap() {
	err := c.cmd.Wait()
	c.mu.Lock()
	if err != nil {
		var exitErr *stdexec.ExitError
		if !errors.As(err, &exitErr) {
			c.waitErr = object.WrapNativeError(object.IOError, `wait() "`+c.argv[0]+`" failed`, err)
		}
	}
	c.result = newResult(c.cmd.ProcessState.ExitCode(), nil, nil)
	c.mu.Unlock()
	close(c.done)
}

func (c *Command) Constructor() *object.Function { return commandType.Function }

func (c *Command) String() string {
	quoted := make([]string, len(c.argv))
	for i, a := range c.argv {
		quoted[i] = strconv.Quote(a)
	}
	return "<Command [" + strings.Join(quoted, ", ") + "]>"
}

func (c *Command) ToString() (string, error) { return c.String(), nil }

func (c *Command) running() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// wait waits for the process to end. Under check=true, the default, a
// non-zero exit raises ExitError as run does; check=false returns the
// Result. Waiting again gives the same outcome.
func (c *Command) wait(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("wait", args)
	timeout, limited := p.Timeout("timeout")
	check := bool(p.BoolOr("check", true))
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if limited {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-c.done:
		case <-timer.C:
			return nil, object.NewTimeoutError("wait() timed out after %g seconds", timeout.Seconds())
		}
	} else {
		<-c.done
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.waitErr != nil {
		return nil, c.waitErr
	}
	if check && c.result.code != 0 {
		return nil, exitError("wait", c.argv[0], c.result, "")
	}
	return c.result, nil
}

func (c *Command) kill(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoArgs("kill", args); err != nil {
		return nil, err
	}
	if !c.running() {
		return object.Nil, nil
	}
	if err := c.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return nil, object.WrapNativeError(object.IOError, `kill() cannot kill "`+c.argv[0]+`"`, err)
	}
	return object.Nil, nil
}

func (c *Command) GetAttr(name string) (object.Object, error) {
	switch name {
	case "pid":
		return object.Integer(c.cmd.Process.Pid), nil
	case "running":
		return object.Bool(c.running()), nil
	case "stdin":
		return c.stdin, nil
	case "stdout":
		return c.stdout, nil
	case "stderr":
		return c.stderr, nil
	case "wait":
		return &object.Function{Name: "wait", Fn: c.wait}, nil
	case "kill":
		return &object.Function{Name: "kill", Fn: c.kill}, nil
	}
	return nil, object.NewAttributeError("Command has no attribute '%s'", name)
}

func (c *Command) Attributes() []string {
	return []string{"pid", "running", "stdin", "stdout", "stderr", "wait", "kill"}
}
