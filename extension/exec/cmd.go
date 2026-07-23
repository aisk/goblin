package exec

import (
	"fmt"
	stdexec "os/exec"

	"github.com/aisk/goblin/object"
)

type Cmd struct {
	objectBase
	cmd *stdexec.Cmd
}

func (c *Cmd) String() string            { return fmt.Sprintf("<exec.Cmd %q>", c.cmd.Args) }
func (c *Cmd) ToString() (string, error) { return c.String(), nil }

func (c *Cmd) run(args object.CallArgs) (object.Object, error) {
	if err := noArgs("run", args); err != nil {
		return nil, err
	}
	if err := c.cmd.Run(); err != nil {
		return nil, commandError("run", err)
	}
	return object.Nil, nil
}

func (c *Cmd) start(args object.CallArgs) (object.Object, error) {
	if err := noArgs("start", args); err != nil {
		return nil, err
	}
	if err := c.cmd.Start(); err != nil {
		return nil, commandError("start", err)
	}
	return object.Nil, nil
}

func (c *Cmd) wait(args object.CallArgs) (object.Object, error) {
	if err := noArgs("wait", args); err != nil {
		return nil, err
	}
	if err := c.cmd.Wait(); err != nil {
		return nil, commandError("wait", err)
	}
	return object.Nil, nil
}

func (c *Cmd) output(args object.CallArgs) (object.Object, error) {
	if err := noArgs("output", args); err != nil {
		return nil, err
	}
	value, err := c.cmd.Output()
	if err != nil {
		return nil, commandError("output", err)
	}
	return object.NewBytes(value), nil
}

func (c *Cmd) combinedOutput(args object.CallArgs) (object.Object, error) {
	if err := noArgs("combined_output", args); err != nil {
		return nil, err
	}
	value, err := c.cmd.CombinedOutput()
	if err != nil {
		return nil, commandError("combined_output", err)
	}
	return object.NewBytes(value), nil
}

func commandError(name string, err error) error {
	return object.WrapNativeError(object.IOError, name+"() failed", err)
}

func (c *Cmd) GetAttr(name string) (object.Object, error) {
	methods := map[string]func(object.CallArgs) (object.Object, error){
		"run":             c.run,
		"start":           c.start,
		"wait":            c.wait,
		"output":          c.output,
		"combined_output": c.combinedOutput,
	}
	if name == "attributes" {
		return object.AttributesFunction(c), nil
	}
	if fn, ok := methods[name]; ok {
		return &object.Function{Name: name, Fn: fn}, nil
	}
	return nil, object.NewAttributeError("Cmd has no attribute '%s'", name)
}

func (c *Cmd) Attributes() []string {
	return []string{"attributes", "run", "start", "wait", "output", "combined_output"}
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

var _ object.Object = (*Cmd)(nil)
