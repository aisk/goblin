package fs

import (
	"io"
	"os"

	pathext "github.com/aisk/goblin/extension/path"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return &object.Module{
		Name: "fs",
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
	ap := object.NewArgParser(funcName, args)
	pathArg := ap.Any("path")
	if err := ap.Finish(); err != nil {
		return "", err
	}
	path, ok := pathext.PathString(pathArg)
	if !ok {
		return "", object.NewTypeError("%s() argument 'path' must be a string or Path, got %s", funcName, pathArg.TypeName())
	}
	return path, nil
}

func openFile(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("open", args)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "open() failed to open file", err)
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
		return nil, object.WrapNativeError(object.IOError, "create() failed to create file", err)
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
		return nil, object.WrapNativeError(object.IOError, "read() failed to open file", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "read() failed to read file", err)
	}
	return object.String(data), nil
}

func bindPathContentArgs(funcName string, args object.CallArgs) (string, string, error) {
	ap := object.NewArgParser(funcName, args)
	pathArg := ap.Any("path")
	contentArg := ap.Any("content")
	if err := ap.Finish(); err != nil {
		return "", "", err
	}
	path, ok := pathext.PathString(pathArg)
	if !ok {
		return "", "", object.NewTypeError("%s() argument 'path' must be a string or Path, got %s", funcName, pathArg.TypeName())
	}
	content, ok := contentArg.(object.String)
	if !ok {
		return "", "", object.NewTypeError("%s() argument 'content' must be a string, got %s", funcName, contentArg.TypeName())
	}
	return path, string(content), nil
}

func writeFile(args object.CallArgs) (object.Object, error) {
	path, content, err := bindPathContentArgs("write", args)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write() failed to write file", err)
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
		return nil, object.WrapNativeError(object.IOError, "append() failed to open file", err)
	}
	defer file.Close()

	n, err := file.WriteString(content)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "append() failed to write file", err)
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
	return nil, object.WrapNativeError(object.IOError, "exists() failed to stat path", err)
}

func stat(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("stat", args)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "stat() failed to stat path", err)
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
		return nil, object.WrapNativeError(object.IOError, "read_dir() failed to read directory", err)
	}

	items := make([]object.Object, len(entries))
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, "read_dir() failed to read directory", err)
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
		return nil, object.WrapNativeError(object.IOError, "mkdir() failed to create directory", err)
	}
	return object.Nil, nil
}

func remove(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("remove", args)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(path); err != nil {
		return nil, object.WrapNativeError(object.IOError, "remove() failed to remove path", err)
	}
	return object.Nil, nil
}
