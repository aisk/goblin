package fs

import (
	"fmt"
	"os"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"open": &object.Function{Name: "open", Fn: openFile},
		},
	}, nil
}

func openFile(args object.CallArgs) (object.Object, error) {
	bound, err := object.BindArguments("open", []string{"path"}, "", "", args)
	if err != nil {
		return nil, err
	}

	path, ok := bound["path"].(object.String)
	if !ok {
		return nil, fmt.Errorf("open() argument must be a string, got %T", bound["path"])
	}

	file, err := os.DirFS(".").Open(string(path))
	if err != nil {
		return nil, fmt.Errorf("open() failed: %w", err)
	}

	return NewFile(string(path), file), nil
}
