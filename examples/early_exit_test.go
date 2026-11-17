package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aisk/goblin/examples/closeprobe"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

// earlyExitDir holds the program exercising every way a for loop can leave a
// pass (the end of it, break, return, an error from the body or from the
// iterator, a return out of nested loops) over close_probe, whose passes
// print when they are closed. It lives outside the examples directory proper
// because close_probe exists only while these tests register it.
const earlyExitDir = "closeprobe/testdata"

// TestEarlyExitInterpreter checks that the interpreter closes each pass
// exactly once, at the right moment.
func TestEarlyExitInterpreter(t *testing.T) {
	closeprobe.Register()
	defer closeprobe.Unregister()
	stdout, stderr := runInterpreter(t, filepath.Join(earlyExitDir, "early_exit.goblin"))
	checkOutput(t, earlyExitDir, "early_exit", ".stdout", stdout, true)
	checkOutput(t, earlyExitDir, "early_exit", ".stderr", stderr, false)
}

// TestEarlyExitTranspiler checks the same program compiled to an executable,
// which registers close_probe from an extra file in its main package.
func TestEarlyExitTranspiler(t *testing.T) {
	closeprobe.Register()
	defer closeprobe.Unregister()
	file := filepath.Join(earlyExitDir, "early_exit.goblin")
	mod, err := source.LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := transpiler.TranspileToDir(mod, file, dir); err != nil {
		t.Fatal(err)
	}
	register := "package main\n\nimport \"github.com/aisk/goblin/examples/closeprobe\"\n\nfunc init() { closeprobe.Register() }\n"
	if err := os.WriteFile(filepath.Join(dir, "closeprobe.go"), []byte(register), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "early_exit")
	build := exec.Command("go", "build", "-mod=mod", "-o", exe, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	var stdout, stderr bytes.Buffer
	run := exec.Command(exe)
	run.Stdout, run.Stderr = &stdout, &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("executable failed: %v\nstderr: %s", err, stderr.String())
	}
	checkOutput(t, earlyExitDir, "early_exit", ".stdout", stdout.String(), true)
	checkOutput(t, earlyExitDir, "early_exit", ".stderr", stderr.String(), false)
}
