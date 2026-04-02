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
