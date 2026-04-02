package fs

import (
	"fmt"
	"io"
	"os"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"open":     &object.Function{Name: "open", Fn: openFile},
			"read":     &object.Function{Name: "read", Fn: readFile},
			"exists":   &object.Function{Name: "exists", Fn: exists},
			"stat":     &object.Function{Name: "stat", Fn: stat},
			"read_dir": &object.Function{Name: "read_dir", Fn: readDir},
		},
	}, nil
}

func bindPathArg(funcName string, args object.CallArgs) (string, error) {
	bound, err := object.BindArguments(funcName, []string{"path"}, "", "", args)
	if err != nil {
		return "", err
	}

	path, ok := bound["path"].(object.String)
	if !ok {
		return "", fmt.Errorf("%s() argument must be a string, got %T", funcName, bound["path"])
	}
	return string(path), nil
}

func openFile(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("open", args)
	if err != nil {
		return nil, err
	}

	file, err := os.DirFS(".").Open(path)
	if err != nil {
		return nil, fmt.Errorf("open() failed: %w", err)
	}

	return NewFile(path, file), nil
}

func readFile(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("read", args)
	if err != nil {
		return nil, err
	}

	file, err := os.DirFS(".").Open(path)
	if err != nil {
		return nil, fmt.Errorf("read() failed: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read() failed: %w", err)
	}
	return object.String(data), nil
}

func exists(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("exists", args)
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(path)
	if err == nil {
		return object.Bool(true), nil
	}
	if os.IsNotExist(err) {
		return object.Bool(false), nil
	}
	return nil, fmt.Errorf("exists() failed: %w", err)
}

func stat(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("stat", args)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat() failed: %w", err)
	}
	return NewFileInfo(info), nil
}

func readDir(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("read_dir", args)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read_dir() failed: %w", err)
	}

	items := make([]object.Object, len(entries))
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read_dir() failed: %w", err)
		}
		items[i] = NewFileInfo(info)
	}
	return &object.List{Elements: items}, nil
}
