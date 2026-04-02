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
