package exec

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	stdexec "os/exec"
	"sync"

	"github.com/aisk/goblin/object"
)

type cmdState int

const (
	stateCreated cmdState = iota
	stateRunning
	stateExited
	stateFailed
)

type Cmd struct {
	objectBase
	mu      sync.Mutex
	cmd     *stdexec.Cmd
	state   cmdState
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
	result  *Result
	waitErr error
	done    chan struct{}
}

func (c *Cmd) String() string            { return fmt.Sprintf("<exec.Cmd %s>", commandString(c.cmd)) }
func (c *Cmd) ToString() (string, error) { return c.String(), nil }

func (c *Cmd) start(args object.CallArgs) (object.Object, error) {
	if err := noArgs("start", args); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != stateCreated {
		return nil, object.NewValueError("start() called after command execution began")
	}
	if err := c.cmd.Start(); err != nil {
		c.state = stateFailed
		return nil, object.WrapNativeError(object.IOError, "start() failed", err)
	}
	c.state = stateRunning
	c.done = make(chan struct{})
	go c.reap()
	return object.Nil, nil
}

func (c *Cmd) reap() {
	err := c.cmd.Wait()
	c.mu.Lock()
	if err != nil {
		var exitErr *stdexec.ExitError
		if !errors.As(err, &exitErr) {
			c.waitErr = object.WrapNativeError(object.IOError, "wait() failed", err)
		}
	}
	c.result = c.makeResult()
	c.state = stateExited
	close(c.done)
	c.mu.Unlock()
}

func (c *Cmd) wait(args object.CallArgs) (object.Object, error) {
	if err := noArgs("wait", args); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.state == stateCreated {
		c.mu.Unlock()
		return nil, object.NewValueError("wait() called before start()")
	}
	if c.state == stateFailed {
		c.mu.Unlock()
		return nil, object.NewValueError("wait() called on a command that failed to start")
	}
	done := c.done
	c.mu.Unlock()
	<-done
	c.mu.Lock()
	result, err := c.result, c.waitErr
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Cmd) run(args object.CallArgs) (object.Object, error) {
	if err := noArgs("run", args); err != nil {
		return nil, err
	}
	if _, err := c.start(object.CallArgs{}); err != nil {
		return nil, err
	}
	return c.wait(object.CallArgs{})
}

func (c *Cmd) kill(args object.CallArgs) (object.Object, error) {
	if err := noArgs("kill", args); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == stateCreated || c.state == stateFailed {
		return nil, object.NewValueError("kill() called before start()")
	}
	if c.state == stateExited {
		return object.Nil, nil
	}
	if err := c.cmd.Process.Kill(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return object.Nil, nil
		}
		return nil, object.WrapNativeError(object.IOError, "kill() failed", err)
	}
	return object.Nil, nil
}

func (c *Cmd) makeResult() *Result {
	var stdout, stderr object.Object = object.Nil, object.Nil
	if c.stdout != nil {
		stdout = object.NewBytes(c.stdout.Bytes())
	}
	if c.stderr != nil {
		stderr = object.NewBytes(c.stderr.Bytes())
	}
	return &Result{objectBase: objectBase{typeName: "Result"}, code: c.cmd.ProcessState.ExitCode(), stdout: stdout, stderr: stderr}
}

func (c *Cmd) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(c), nil
	case "run":
		return &object.Function{Name: "run", Fn: c.run}, nil
	case "start":
		return &object.Function{Name: "start", Fn: c.start}, nil
	case "wait":
		return &object.Function{Name: "wait", Fn: c.wait}, nil
	case "kill":
		return &object.Function{Name: "kill", Fn: c.kill}, nil
	case "pid":
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.cmd.Process == nil {
			return object.Nil, nil
		}
		return object.Integer(c.cmd.Process.Pid), nil
	case "running":
		c.mu.Lock()
		defer c.mu.Unlock()
		return object.Bool(c.state == stateRunning), nil
	}
	return nil, object.NewAttributeError("Cmd has no attribute '%s'", name)
}
func (c *Cmd) Attributes() []string {
	return []string{"attributes", "run", "start", "wait", "kill", "pid", "running"}
}

func noArgs(name string, args object.CallArgs) error {
	if err := object.RequireNoKeyword(name, args); err != nil {
		return err
	}
	if len(args.Positional) != 0 {
		return object.NewTypeError("%s() takes no arguments, got %d", name, len(args.Positional))
	}
	return nil
}
