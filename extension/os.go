package extension

import (
	"fmt"
	"os"

	"github.com/aisk/goblin/object"
)

var OsModule = &object.Module{
	Members: map[string]object.Object{
		"exit":    &object.Function{Name: "exit", Fn: exit},
		"getenv":  &object.Function{Name: "getenv", Fn: getenv},
		"getpid":  &object.Function{Name: "getpid", Fn: getpid},
		"getppid": &object.Function{Name: "getppid", Fn: getppid},
		"getuid":  &object.Function{Name: "getuid", Fn: getuid},
	},
}

func exit(args object.Args, kwargs object.KwArgs) (object.Object, error) {
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

func getenv(args object.Args, kwargs object.KwArgs) (object.Object, error) {
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

func getpid(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("getpid() requires no arguments")
	}
	pid := os.Getpid()
	return object.Integer(pid), nil
}

func getppid(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("getppid() requires no arguments")
	}
	ppid := os.Getppid()
	return object.Integer(ppid), nil
}

func getuid(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("getuid() requires no arguments")
	}
	uid := os.Getuid()
	return object.Integer(uid), nil
}
