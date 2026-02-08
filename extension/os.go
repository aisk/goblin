package extension

import (
	"fmt"
	"os"

	"github.com/aisk/goblin/object"
)

var OsModule = &object.Module{
	Members: map[string]object.Object{
		"exit":    &object.Function{Name: "exit", Fn: Exit},
		"getenv":  &object.Function{Name: "getenv", Fn: Getenv},
		"getpid":  &object.Function{Name: "getpid", Fn: Getpid},
		"getppid": &object.Function{Name: "getppid", Fn: Getppid},
		"getuid":  &object.Function{Name: "getuid", Fn: Getuid},
	},
}

func Exit(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("exit() requires exactly 1 argument")
	}
	code, ok := args[0].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("exit() argument must be an integer")
	}
	os.Exit(int(code))
	return nil, nil
}

func Getenv(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("getenv() requires exactly 1 argument")
	}
	key, ok := args[0].(object.String)
	if !ok {
		return nil, fmt.Errorf("getenv() argument must be a string")
	}
	value := os.Getenv(string(key))
	return object.String(value), nil
}

func Getpid(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("getpid() requires no arguments")
	}
	pid := os.Getpid()
	return object.Integer(pid), nil
}

func Getppid(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("getppid() requires no arguments")
	}
	ppid := os.Getppid()
	return object.Integer(ppid), nil
}

func Getuid(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("getuid() requires no arguments")
	}
	uid := os.Getuid()
	return object.Integer(uid), nil
}
