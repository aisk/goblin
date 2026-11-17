package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aisk/goblin/examples/httpserver"
	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

// httpDir holds the program that runs the http chapter's common tasks. It
// talks to the local server httpserver starts, whose addresses only exist
// while these tests run, so it lives outside the examples directory proper.
const httpDir = "httpserver/testdata"

// startHTTP starts the servers and puts their addresses in the environment
// the program reads them from.
func startHTTP(t *testing.T) []string {
	t.Helper()
	plain, tls := httpserver.Start()
	t.Cleanup(plain.Close)
	t.Cleanup(tls.Close)
	return []string{"GOBLIN_HTTP_TEST_URL=" + plain.URL, "GOBLIN_HTTPS_TEST_URL=" + tls.URL}
}

// TestHTTPTasksInterpreter runs the http tasks through the interpreter.
func TestHTTPTasksInterpreter(t *testing.T) {
	for _, kv := range startHTTP(t) {
		name, value, _ := cut(kv)
		t.Setenv(name, value)
	}
	stdout, stderr := runInterpreter(t, filepath.Join(httpDir, "http_tasks.goblin"))
	checkOutput(t, httpDir, "http_tasks", ".stdout", stdout, true)
	checkOutput(t, httpDir, "http_tasks", ".stderr", stderr, false)
}

// TestHTTPTasksTranspiler runs the same program compiled to an executable.
func TestHTTPTasksTranspiler(t *testing.T) {
	env := startHTTP(t)
	file := filepath.Join(httpDir, "http_tasks.goblin")
	mod, err := source.LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var buildLog bytes.Buffer
	exe, err := transpiler.BuildExecutable(mod, file, transpiler.BuildOptions{
		Output: filepath.Join(t.TempDir(), "http_tasks"),
		Stdout: &buildLog,
		Stderr: &buildLog,
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, buildLog.String())
	}
	var stdout, stderr bytes.Buffer
	run := exec.Command(exe)
	run.Env = append(os.Environ(), env...)
	run.Stdout, run.Stderr = &stdout, &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("executable failed: %v\nstderr: %s", err, stderr.String())
	}
	checkOutput(t, httpDir, "http_tasks", ".stdout", stdout.String(), true)
	checkOutput(t, httpDir, "http_tasks", ".stderr", stderr.String(), false)
}

func cut(kv string) (string, string, bool) {
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i], kv[i+1:], true
		}
	}
	return kv, "", false
}
