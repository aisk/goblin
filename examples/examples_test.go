package examples_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

// TestExamples compiles each example the way `goblin build-exe` does, runs the
// executable, and compares its output against the golden files.
func TestExamples(t *testing.T) {
	for _, name := range exampleNames(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr := buildAndRun(t, name, false)
			checkOutput(t, ".", name, ".stdout", stdout, true)
			checkOutput(t, ".", name, ".stderr", stderr, false)
		})
	}
}

// concurrencyMarkers matches the constructs that start goroutines or use
// channels; any example using one is also run under the Go race detector.
// Detection by content means a new concurrency example cannot silently skip
// the race run.
var concurrencyMarkers = regexp.MustCompile(`\bspawn\b|\bChan\(|\bGoblin\(`)

// TestExamplesRace runs the concurrency examples under the race detector,
// guarding the coordination patterns the book recommends.
func TestExamplesRace(t *testing.T) {
	for _, name := range exampleNames(t) {
		src, err := os.ReadFile(name + ".goblin")
		if err != nil {
			t.Fatal(err)
		}
		if !concurrencyMarkers.Match(src) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr := buildAndRun(t, name, true)
			checkOutput(t, ".", name, ".stdout", stdout, true)
			checkOutput(t, ".", name, ".stderr", stderr, false)
		})
	}
}

// exampleNames lists the examples in this directory, without extension.
func exampleNames(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.goblin")
	if err != nil || len(files) == 0 {
		t.Fatalf("no .goblin files found: %v", err)
	}
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = strings.TrimSuffix(file, ".goblin")
	}
	return names
}

// buildAndRun compiles an example to an executable and runs it in this
// directory, returning its stdout and stderr.
func buildAndRun(t *testing.T, name string, race bool) (string, string) {
	t.Helper()
	file := name + ".goblin"
	mod, err := source.LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var buildLog bytes.Buffer
	exe, err := transpiler.BuildExecutable(mod, file, transpiler.BuildOptions{
		Output: filepath.Join(t.TempDir(), name),
		Race:   race,
		Stdout: &buildLog,
		Stderr: &buildLog,
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, buildLog.String())
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("executable failed: %v\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// checkOutput compares actual output against the expected file.
// A stdout golden file is required. A missing stderr file means empty stderr.
func checkOutput(t *testing.T, examplesDir, baseName, suffix, actual string, required bool) {
	t.Helper()

	expectedFile := filepath.Join(examplesDir, baseName+suffix)

	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		if required {
			t.Fatalf("missing expected output file: %s", expectedFile)
		}
		if normalize(actual) != "" {
			t.Errorf("%s mismatch (no expected file, assuming empty):\nActual:\n%s", suffix, actual)
		}
		return
	}

	expected, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read %s file: %v", suffix, err)
	}

	if normalize(actual) != normalize(string(expected)) {
		t.Errorf("%s mismatch:\nExpected:\n%s\nActual:\n%s", suffix, string(expected), actual)
	}
}

// normalize standardizes line endings for cross-platform comparison.
func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
