package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func fsFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("Execute() returned %T", modObj)
	}

	member, ok := mod.Members[name]
	if !ok {
		t.Fatalf("fs module missing %q", name)
	}

	fn, ok := member.(*object.Function)
	if !ok {
		t.Fatalf("fs module member %q is %T", name, member)
	}

	return fn
}

func TestFsOpenReadClose(t *testing.T) {
	tempDir := t.TempDir()
	fileName := "fixture.txt"
	filePath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(filePath, []byte("hello from fs"), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	fileObj, err := fsFunction(t, "open").Call(object.CallArgs{
		Positional: object.Args{object.String(fileName)},
	})
	if err != nil {
		t.Fatalf("open() error = %v", err)
	}

	file, ok := fileObj.(*File)
	if !ok {
		t.Fatalf("open() returned %T", fileObj)
	}

	contentObj, err := file.Read(object.CallArgs{})
	if err != nil {
		t.Fatalf("read() error = %v", err)
	}
	if got := contentObj.String(); got != "hello from fs" {
		t.Fatalf("read() = %q, want %q", got, "hello from fs")
	}

	if _, err := file.Close(object.CallArgs{}); err != nil {
		t.Fatalf("close() error = %v", err)
	}

	if _, err := file.Read(object.CallArgs{}); err == nil || !strings.Contains(err.Error(), "closed file") {
		t.Fatalf("read() after close error = %v, want closed file error", err)
	}
}

func TestFsAcceptsPath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "fixture.txt")
	if err := os.WriteFile(filePath, []byte("via path"), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	// A Path argument must be accepted anywhere a path string is, so fs and the
	// path module's Path type interoperate without manual string conversion.
	readObj, err := fsFunction(t, "read").Call(object.CallArgs{
		Positional: object.Args{object.NewPath(filePath)},
	})
	if err != nil {
		t.Fatalf("read(Path) error = %v", err)
	}
	if got := readObj.String(); got != "via path" {
		t.Fatalf("read(Path) = %q, want %q", got, "via path")
	}

	existsObj, err := fsFunction(t, "exists").Call(object.CallArgs{
		Positional: object.Args{object.NewPath(filePath)},
	})
	if err != nil {
		t.Fatalf("exists(Path) error = %v", err)
	}
	if existsObj != object.True {
		t.Fatalf("exists(Path) = %v, want true", existsObj)
	}
}

func TestFsHelpers(t *testing.T) {
	tempDir := t.TempDir()
	fileName := "fixture.txt"
	filePath := filepath.Join(tempDir, fileName)
	dirName := "nested"
	dirPath := filepath.Join(tempDir, dirName)

	if err := os.WriteFile(filePath, []byte("helper text"), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("os.Mkdir() error = %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	readObj, err := fsFunction(t, "read").Call(object.CallArgs{
		Positional: object.Args{object.String(fileName)},
	})
	if err != nil {
		t.Fatalf("read() error = %v", err)
	}
	if got := readObj.String(); got != "helper text" {
		t.Fatalf("read() = %q, want %q", got, "helper text")
	}

	existsObj, err := fsFunction(t, "exists").Call(object.CallArgs{
		Positional: object.Args{object.String(fileName)},
	})
	if err != nil {
		t.Fatalf("exists() error = %v", err)
	}
	if got := existsObj.String(); got != "true" {
		t.Fatalf("exists() = %q, want %q", got, "true")
	}

	missingObj, err := fsFunction(t, "exists").Call(object.CallArgs{
		Positional: object.Args{object.String("missing.txt")},
	})
	if err != nil {
		t.Fatalf("exists(missing) error = %v", err)
	}
	if got := missingObj.String(); got != "false" {
		t.Fatalf("exists(missing) = %q, want %q", got, "false")
	}

	statObj, err := fsFunction(t, "stat").Call(object.CallArgs{
		Positional: object.Args{object.String(fileName)},
	})
	if err != nil {
		t.Fatalf("stat() error = %v", err)
	}
	statInfo, ok := statObj.(*FileInfo)
	if !ok {
		t.Fatalf("stat() returned %T", statObj)
	}
	if statInfo.Info.Name() != fileName {
		t.Fatalf("stat().name = %q, want %q", statInfo.Info.Name(), fileName)
	}
	if statInfo.Info.Size() != int64(len("helper text")) {
		t.Fatalf("stat().size = %d, want %d", statInfo.Info.Size(), len("helper text"))
	}

	listObj, err := fsFunction(t, "read_dir").Call(object.CallArgs{
		Positional: object.Args{object.String(".")},
	})
	if err != nil {
		t.Fatalf("read_dir() error = %v", err)
	}
	list, ok := listObj.(*object.List)
	if !ok {
		t.Fatalf("read_dir() returned %T", listObj)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("read_dir() length = %d, want 2", len(list.Elements))
	}

	first, ok := list.Elements[0].(*FileInfo)
	if !ok {
		t.Fatalf("read_dir()[0] is %T", list.Elements[0])
	}
	second, ok := list.Elements[1].(*FileInfo)
	if !ok {
		t.Fatalf("read_dir()[1] is %T", list.Elements[1])
	}
	if first.Info.Name() != fileName || second.Info.Name() != dirName {
		t.Fatalf("read_dir() names = %q, %q; want %q, %q", first.Info.Name(), second.Info.Name(), fileName, dirName)
	}
}

func TestFsWriteCreateAppendMkdirRemove(t *testing.T) {
	tempDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	fileObj, err := fsFunction(t, "create").Call(object.CallArgs{
		Positional: object.Args{object.String("created.txt")},
	})
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	file, ok := fileObj.(*File)
	if !ok {
		t.Fatalf("create() returned %T", fileObj)
	}
	if _, err := file.Write(object.CallArgs{Positional: object.Args{object.String("hello")}}); err != nil {
		t.Fatalf("file.write() error = %v", err)
	}
	if _, err := file.Close(object.CallArgs{}); err != nil {
		t.Fatalf("file.close() error = %v", err)
	}
	content, err := os.ReadFile("created.txt")
	if err != nil {
		t.Fatalf("os.ReadFile(created.txt) error = %v", err)
	}
	if got := string(content); got != "hello" {
		t.Fatalf("created.txt = %q, want %q", got, "hello")
	}

	if _, err := fsFunction(t, "write").Call(object.CallArgs{
		Positional: object.Args{object.String("written.txt"), object.String("abc")},
	}); err != nil {
		t.Fatalf("write() error = %v", err)
	}
	if _, err := fsFunction(t, "append").Call(object.CallArgs{
		Positional: object.Args{object.String("written.txt"), object.String("def")},
	}); err != nil {
		t.Fatalf("append() error = %v", err)
	}
	content, err = os.ReadFile("written.txt")
	if err != nil {
		t.Fatalf("os.ReadFile(written.txt) error = %v", err)
	}
	if got := string(content); got != "abcdef" {
		t.Fatalf("written.txt = %q, want %q", got, "abcdef")
	}

	if _, err := fsFunction(t, "mkdir").Call(object.CallArgs{
		Positional: object.Args{object.String("made_dir")},
	}); err != nil {
		t.Fatalf("mkdir() error = %v", err)
	}
	info, err := os.Stat("made_dir")
	if err != nil {
		t.Fatalf("os.Stat(made_dir) error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("made_dir should be a directory")
	}

	if _, err := fsFunction(t, "remove").Call(object.CallArgs{
		Positional: object.Args{object.String("written.txt")},
	}); err != nil {
		t.Fatalf("remove(file) error = %v", err)
	}
	if _, err := os.Stat("written.txt"); !os.IsNotExist(err) {
		t.Fatalf("written.txt should be removed, stat err = %v", err)
	}

	if _, err := fsFunction(t, "remove").Call(object.CallArgs{
		Positional: object.Args{object.String("made_dir")},
	}); err != nil {
		t.Fatalf("remove(dir) error = %v", err)
	}
	if _, err := os.Stat("made_dir"); !os.IsNotExist(err) {
		t.Fatalf("made_dir should be removed, stat err = %v", err)
	}
}
