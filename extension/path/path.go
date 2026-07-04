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
		Members: map[string]object.Object{
			"Path": object.PathConstructorFn,
			"cwd":  &object.Function{Name: "cwd", Fn: cwd},
			"home": &object.Function{Name: "home", Fn: home},
		},
	}, nil
}

func cwd(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("cwd", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("cwd() takes no arguments, got %d", len(args.Positional))
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "cwd() failed", err)
	}
	return object.NewPath(dir), nil
}

func home(args object.CallArgs) (object.Object, error) {
	if err := object.RequireNoKeyword("home", args); err != nil {
		return nil, err
	}
	if len(args.Positional) != 0 {
		return nil, object.NewTypeError("home() takes no arguments, got %d", len(args.Positional))
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "home() failed", err)
	}
	return object.NewPath(dir), nil
}
