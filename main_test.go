package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWantsHelp(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"-h"}, want: true},
		{args: []string{"--help"}, want: true},
		{args: []string{"-h", "script.goblin"}, want: false},
		{args: []string{"--help", "script.goblin"}, want: false},
		{args: []string{"script.goblin", "-h"}, want: false},
		{args: []string{"script.goblin"}, want: false},
	}
	for _, tt := range tests {
		if got := wantsHelp(tt.args); got != tt.want {
			t.Errorf("wantsHelp(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestRequireSourceFirst(t *testing.T) {
	if err := requireSourceFirst([]string{"script.goblin", "-h"}); err != nil {
		t.Fatalf("requireSourceFirst(script, -h) = %v, want nil", err)
	}
	if err := requireSourceFirst([]string{"-h", "script.goblin"}); err == nil {
		t.Fatal("should not accept leading -h before source")
	}
	if err := requireSourceFirst([]string{"--verbose"}); err == nil {
		t.Fatal("should not accept --verbose without source")
	}
}

func TestRunCLIForwardsScriptFlags(t *testing.T) {
	bin := sharedGoblinBin(t)
	script := filepath.Join(t.TempDir(), "argv.goblin")
	if err := os.WriteFile(script, []byte(`import "os"
for a in os.argv() {
    print(a)
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(bin, "run", script, "--verbose", "-h").CombinedOutput()
	if err != nil {
		t.Fatalf("goblin run: %v\n%s", err, out)
	}
	want := script + "\n--verbose\n-h\n"
	if strings.ReplaceAll(string(out), "\r\n", "\n") != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

func TestBuiltExecutableForwardsArguments(t *testing.T) {
	bin := sharedGoblinBin(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "argv.goblin")
	executable := filepath.Join(dir, "argv-program")
	if err := os.WriteFile(script, []byte(`import "os"
var args = os.argv()
print(args[1])
print(args[2])
`), 0644); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command(bin, "build-exe", "-o", executable, script).CombinedOutput(); err != nil {
		t.Fatalf("goblin build-exe: %v\n%s", err, out)
	}
	out, err := exec.Command(executable, "foo", "--bar").CombinedOutput()
	if err != nil {
		t.Fatalf("compiled program: %v\n%s", err, out)
	}
	if got, want := strings.ReplaceAll(string(out), "\r\n", "\n"), "foo\n--bar\n"; got != want {
		t.Fatalf("compiled stdout = %q, want %q", got, want)
	}
}

// The generated Go module is named after the source file, which must not
// collide with a standard library package path.
func TestBuiltExecutableNamedAfterStdlibPackage(t *testing.T) {
	bin := sharedGoblinBin(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "io.goblin")
	executable := filepath.Join(dir, "io-program")
	if err := os.WriteFile(script, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "build-exe", "-o", executable, script).CombinedOutput(); err != nil {
		t.Fatalf("goblin build-exe: %v\n%s", err, out)
	}
	out, err := exec.Command(executable).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled program: %v\n%s", err, out)
	}
	if got := strings.ReplaceAll(string(out), "\r\n", "\n"); got != "ok\n" {
		t.Fatalf("compiled stdout = %q, want %q", got, "ok\n")
	}
}

// Local (path) imports compile into packages of their own, imported by the
// generated main package.
func TestBuiltExecutableWithLocalModuleImport(t *testing.T) {
	bin := sharedGoblinBin(t)
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper.goblin")
	if err := os.WriteFile(helper, []byte(`import "math"
var greeting = "hi"
func double(x) {
    return math.abs(x) * 2
}
export double
export greeting
`), 0644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "main.goblin")
	if err := os.WriteFile(script, []byte(`import "./helper"
import "x/encoding/hex"
print(helper.double(-21))
print(helper.greeting)
print(hex.encode("a"))
`), 0644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "local-import-program")

	if out, err := exec.Command(bin, "build-exe", "-o", executable, script).CombinedOutput(); err != nil {
		t.Fatalf("goblin build-exe: %v\n%s", err, out)
	}
	out, err := exec.Command(executable).CombinedOutput()
	if err != nil {
		t.Fatalf("compiled program: %v\n%s", err, out)
	}
	if got, want := strings.ReplaceAll(string(out), "\r\n", "\n"), "42\nhi\n61\n"; got != want {
		t.Fatalf("compiled stdout = %q, want %q", got, want)
	}
}

func TestRunCLIRejectsLeadingFlag(t *testing.T) {
	bin := sharedGoblinBin(t)
	out, err := exec.Command(bin, "run", "-h", "script.goblin").CombinedOutput()
	if err == nil {
		t.Fatalf("should not accept leading -h before source, got output:\n%s", out)
	}
	// Exit status alone is not enough: if requireSourceFirst were skipped,
	// "-h" would be treated as the source path and fail with a file-open
	// error (still non-zero), so the test would pass for the wrong reason.
	if !strings.Contains(string(out), "flag-like") {
		t.Fatalf("goblin run -h script: wrong failure, want flag-like rejection, got %q", out)
	}
}

var (
	goblinBinOnce sync.Once
	goblinBinDir  string
	goblinBinPath string
	goblinBinErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if goblinBinDir != "" {
		if err := os.RemoveAll(goblinBinDir); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", goblinBinDir, err)
			if code == 0 {
				code = 1
			}
		}
	}
	os.Exit(code)
}

func sharedGoblinBin(t *testing.T) string {
	t.Helper()
	goblinBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "goblin-cli-bin-")
		if err != nil {
			goblinBinErr = err
			return
		}
		goblinBinDir = dir
		goblinBinPath = filepath.Join(dir, "goblin")
		cmd := exec.Command("go", "build", "-o", goblinBinPath, ".")
		if output, err := cmd.CombinedOutput(); err != nil {
			goblinBinErr = fmt.Errorf("go build: %v\n%s", err, output)
		}
	})
	if goblinBinErr != nil {
		t.Fatal(goblinBinErr)
	}
	return goblinBinPath
}
