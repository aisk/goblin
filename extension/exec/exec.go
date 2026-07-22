package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	stdexec "os/exec"

	"github.com/aisk/goblin/object"
)

type streamPolicy struct {
	objectBase
	name string
}

func (p *streamPolicy) String() string            { return "exec." + p.name }
func (p *streamPolicy) ToString() (string, error) { return p.String(), nil }
func (p *streamPolicy) GetAttr(name string) (object.Object, error) {
	if name == "attributes" {
		return object.AttributesFunction(p), nil
	}
	return nil, object.NewAttributeError("stream policy has no attribute '%s'", name)
}
func (p *streamPolicy) Attributes() []string { return []string{"attributes"} }

var (
	inherit = &streamPolicy{objectBase: objectBase{typeName: "stream policy"}, name: "INHERIT"}
	discard = &streamPolicy{objectBase: objectBase{typeName: "stream policy"}, name: "DISCARD"}
	capture = &streamPolicy{objectBase: objectBase{typeName: "stream policy"}, name: "CAPTURE"}
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "exec", Members: map[string]object.Object{
		"Command": &object.Function{Name: "Command", Fn: command},
		"INHERIT": inherit,
		"DISCARD": discard,
		"CAPTURE": capture,
	}}, nil
}

func command(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Command", args)
	name := ap.Str("name")
	argsObj := ap.AnyOr("args", &object.List{})
	cwdObj := ap.AnyOr("cwd", object.Nil)
	envObj := ap.AnyOr("env", object.Nil)
	stdinObj := ap.AnyOr("stdin", inherit)
	stdoutObj := ap.AnyOr("stdout", inherit)
	stderrObj := ap.AnyOr("stderr", inherit)
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	list, ok := argsObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("Command() argument 'args' must be a list, got %T", argsObj)
	}
	argv := make([]string, len(list.Elements))
	for i, arg := range list.Elements {
		s, ok := arg.(object.String)
		if !ok {
			return nil, object.NewTypeError("Command() argument 'args' must contain only strings, got %T at index %d", arg, i)
		}
		argv[i] = string(s)
	}

	cmd := stdexec.Command(string(name), argv...)
	if _, ok := cwdObj.(object.Unit); !ok {
		cwd, ok := object.PathString(cwdObj)
		if !ok {
			return nil, object.NewTypeError("Command() argument 'cwd' must be unit, str, or Path, got %T", cwdObj)
		}
		cmd.Dir = cwd
	}
	if _, ok := envObj.(object.Unit); !ok {
		dict, ok := envObj.(*object.Dict)
		if !ok {
			return nil, object.NewTypeError("Command() argument 'env' must be unit or dict, got %T", envObj)
		}
		cmd.Env = make([]string, 0, len(dict.Entries))
		for _, entry := range dict.Entries {
			key, keyOK := entry.Key.(object.String)
			value, valueOK := entry.Value.(object.String)
			if !keyOK || !valueOK {
				return nil, object.NewTypeError("Command() argument 'env' must contain only string keys and values")
			}
			cmd.Env = append(cmd.Env, string(key)+"="+string(value))
		}
	}

	c := &Cmd{objectBase: objectBase{typeName: "Cmd"}, cmd: cmd, state: stateCreated}
	if err := c.configureStdin(stdinObj); err != nil {
		return nil, err
	}
	if err := c.configureOutput("stdout", stdoutObj); err != nil {
		return nil, err
	}
	if err := c.configureOutput("stderr", stderrObj); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Cmd) configureStdin(value object.Object) error {
	switch v := value.(type) {
	case *streamPolicy:
		switch v {
		case inherit:
			c.cmd.Stdin = os.Stdin
		case discard:
			c.cmd.Stdin = nil
		default:
			return object.NewTypeError("Command() argument 'stdin' does not accept exec.%s", v.name)
		}
	case object.String:
		c.cmd.Stdin = bytes.NewReader([]byte(v))
	case object.Bytes:
		c.cmd.Stdin = bytes.NewReader([]byte(v))
	default:
		return object.NewTypeError("Command() argument 'stdin' must be INHERIT, DISCARD, str, or Bytes, got %T", value)
	}
	return nil
}

func (c *Cmd) configureOutput(name string, value object.Object) error {
	policy, ok := value.(*streamPolicy)
	if !ok {
		return object.NewTypeError("Command() argument '%s' must be INHERIT, DISCARD, or CAPTURE, got %T", name, value)
	}
	var writer io.Writer
	switch policy {
	case inherit:
		if name == "stdout" {
			writer = os.Stdout
		} else {
			writer = os.Stderr
		}
	case discard:
		writer = io.Discard
	case capture:
		if name == "stdout" {
			c.stdout = &bytes.Buffer{}
			writer = c.stdout
		} else {
			c.stderr = &bytes.Buffer{}
			writer = c.stderr
		}
	default:
		return object.NewTypeError("invalid stream policy %s", policy.String())
	}
	if name == "stdout" {
		c.cmd.Stdout = writer
	} else {
		c.cmd.Stderr = writer
	}
	return nil
}

func commandString(cmd *stdexec.Cmd) string { return fmt.Sprintf("%q", cmd.Args) }
