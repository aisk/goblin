// Package exec adapts Go's os/exec package to Goblin.
package exec

import (
	"bytes"
	stdexec "os/exec"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{Name: "exec", Members: map[string]object.Object{
		"Command":   &object.Function{Name: "Command", Fn: command},
		"look_path": &object.Function{Name: "look_path", Fn: lookPath},
	}}, nil
}

// command combines os/exec.Command with the Goblin-representable Cmd fields.
func command(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("Command", args)
	name := ap.Str("name")
	argsObj := ap.AnyOr("args", &object.List{})
	dirObj := ap.AnyOr("dir", object.Nil)
	envObj := ap.AnyOr("env", object.Nil)
	stdinObj := ap.AnyOr("stdin", object.Nil)
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	list, ok := argsObj.(*object.List)
	if !ok {
		return nil, object.NewTypeError("Command() argument 'args' must be a list, got %T", argsObj)
	}
	argv := make([]string, len(list.Elements))
	for i, arg := range list.Elements {
		value, ok := arg.(object.String)
		if !ok {
			return nil, object.NewTypeError("Command() argument 'args' must contain only strings, got %T at index %d", arg, i)
		}
		argv[i] = string(value)
	}

	cmd := stdexec.Command(string(name), argv...)
	if _, ok := dirObj.(object.Unit); !ok {
		dir, ok := object.PathString(dirObj)
		if !ok {
			return nil, object.NewTypeError("Command() argument 'dir' must be unit, str, or Path, got %T", dirObj)
		}
		cmd.Dir = dir
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
	switch value := stdinObj.(type) {
	case object.Unit:
	case object.String:
		cmd.Stdin = bytes.NewReader([]byte(value))
	case object.Bytes:
		cmd.Stdin = bytes.NewReader([]byte(value))
	default:
		return nil, object.NewTypeError("Command() argument 'stdin' must be unit, str, or Bytes, got %T", stdinObj)
	}
	return &Cmd{objectBase: objectBase{typeName: "Cmd"}, cmd: cmd}, nil
}

func lookPath(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("look_path", args)
	file := ap.Str("file")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	path, err := stdexec.LookPath(string(file))
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "look_path() failed", err)
	}
	return object.String(path), nil
}
