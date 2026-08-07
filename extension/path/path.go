package path

import (
	"os"

	"github.com/aisk/goblin/object"
)

// Execute returns the path module, an object-oriented filesystem path API
// modelled after Python's pathlib. Its centrepiece is the Path type; the
// module-level members are only the factories that have no receiving Path to
// call an instance method on.
func Execute() (object.Object, error) {
	return &object.Module{
		Name: "path",
		Members: map[string]object.Object{
			"Path": PathConstructorFn,
			"cwd":  &object.Function{Name: "cwd", Fn: cwd},
			"home": &object.Function{Name: "home", Fn: home},
		},
	}, nil
}

func cwd(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("cwd", args).Finish(); err != nil {
		return nil, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "cwd() failed", err)
	}
	return NewPath(dir), nil
}

func home(args object.CallArgs) (object.Object, error) {
	if err := object.NewArgParser("home", args).Finish(); err != nil {
		return nil, err
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "home() failed", err)
	}
	return NewPath(dir), nil
}
