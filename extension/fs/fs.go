package fs

import (
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

	path, ok := object.PathString(bound["path"])
	if !ok {
		return "", object.NewTypeError("%s() argument must be a string or Path, got %T", funcName, bound["path"])
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
		return nil, object.WrapNativeError(object.IOError, "open() failed", err)
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
		return nil, object.WrapNativeError(object.IOError, "create() failed", err)
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
		return nil, object.WrapNativeError(object.IOError, "read() failed", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "read() failed", err)
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
		return "", "", object.NewTypeError("%s() path argument must be a string, got %T", funcName, bound["path"])
	}
	content, ok := bound["content"].(object.String)
	if !ok {
		return "", "", object.NewTypeError("%s() content argument must be a string, got %T", funcName, bound["content"])
	}
	return string(path), string(content), nil
}

func writeFile(args object.CallArgs) (object.Object, error) {
	path, content, err := bindPathContentArgs("write", args)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write() failed", err)
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
		return nil, object.WrapNativeError(object.IOError, "append() failed", err)
	}
	defer file.Close()

	n, err := file.WriteString(content)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "append() failed", err)
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
	return nil, object.WrapNativeError(object.IOError, "exists() failed", err)
}

func stat(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("stat", args)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, object.WrapNativeError(object.IOError, "stat() failed", err)
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
		return nil, object.WrapNativeError(object.IOError, "read_dir() failed", err)
	}

	items := make([]object.Object, len(entries))
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, object.WrapNativeError(object.IOError, "read_dir() failed", err)
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
		return nil, object.WrapNativeError(object.IOError, "mkdir() failed", err)
	}
	return object.Nil, nil
}

func remove(args object.CallArgs) (object.Object, error) {
	path, err := bindPathArg("remove", args)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(path); err != nil {
		return nil, object.WrapNativeError(object.IOError, "remove() failed", err)
	}
	return object.Nil, nil
}
