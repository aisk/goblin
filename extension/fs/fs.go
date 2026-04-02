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
			"create":   &object.Function{Name: "create", Fn: createFile},
			"read":     &object.Function{Name: "read", Fn: readFile},
			"write":    &object.Function{Name: "write", Fn: writeFile},
			"append":   &object.Function{Name: "append", Fn: appendFile},
			"exists":   &object.Function{Name: "exists", Fn: exists},
			"stat":     &object.Function{Name: "stat", Fn: stat},
			"read_dir": &object.Function{Name: "read_dir", Fn: readDir},
			"mkdir":    &object.Function{Name: "mkdir", Fn: mkdir},
			"remove":   &object.Function{Name: "remove", Fn: remove},
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

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open() failed: %w", err)
	}

	return NewFile(path, file), nil
}

func createFile(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("create", args)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create() failed: %w", err)
	}
	return NewFile(path, file), nil
}

func readFile(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("read", args)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
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

func bindPathContentArgs(funcName string, args object.CallArgs) (string, string, error) {
	bound, err := object.BindArguments(funcName, []string{"path", "content"}, "", "", args)
	if err != nil {
		return "", "", err
	}

	path, ok := bound["path"].(object.String)
	if !ok {
		return "", "", fmt.Errorf("%s() path argument must be a string, got %T", funcName, bound["path"])
	}
	content, ok := bound["content"].(object.String)
	if !ok {
		return "", "", fmt.Errorf("%s() content argument must be a string, got %T", funcName, bound["content"])
	}
	return string(path), string(content), nil
}

func writeFile(args object.CallArgs) (object.Object, error) {
	path, content, err := bindPathContentArgs("write", args)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write() failed: %w", err)
	}
	return object.Integer(len(content)), nil
}

func appendFile(args object.CallArgs) (object.Object, error) {
	path, content, err := bindPathContentArgs("append", args)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("append() failed: %w", err)
	}
	defer file.Close()

	n, err := file.WriteString(content)
	if err != nil {
		return nil, fmt.Errorf("append() failed: %w", err)
	}
	return object.Integer(n), nil
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

func mkdir(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("mkdir", args)
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(path, 0755); err != nil {
		return nil, fmt.Errorf("mkdir() failed: %w", err)
	}
	return object.Nil, nil
}

func remove(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("remove", args)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("remove() failed: %w", err)
	}
	return object.Nil, nil
}
