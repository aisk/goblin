package examples_test

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

// exitDir holds the programs that call os.exit. The example tests require a
// program to end with status 0 and interpret it inside the test process,
// which os.exit would end, so these run as real processes here.
const exitDir = "exitprocess"

// exitCase is one run of a program in exitDir: its stdout and stderr are
// compared with <name>.stdout and <name>.stderr unless stdout is given.
type exitCase struct {
	name   string
	args   []string
	status int
	stdout *string
	// race also builds the executable with the race detector.
	race bool
	// shell marks a case that runs sh.
	shell bool
}

// TestExitProcess runs each program under goblin run and as a compiled
// executable, checking stdout, stderr and the exit status.
func TestExitProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the goblin binary and executables")
	}
	goblin := filepath.Join(t.TempDir(), "goblin")
	build := exec.Command("go", "build", "-o", goblin, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	cases := []exitCase{
		{name: "unwind", status: 3},
		{name: "defer_raises", status: 4},
		{name: "defer_raises_zero", status: 1},
		{name: "defer_exits", status: 7},
		{name: "error_then_exit", status: 8},
		{name: "zero", status: 0},
		{name: "imported", status: 12},
		{name: "import_exits", status: 13},
		{name: "spawned", status: 14, race: true},
		{name: "main_exits", status: 15, race: true},
	}
	// Each callback case prints the deferred calls of run and of the module.
	for _, which := range []string{
		"sort", "map", "max", "regex", "hash", "gzip", "json_decode", "json_encode",
		"csv", "for", "list", "interpolate", "print", "eq", "walk", "conv",
		"tar_read", "tar_write", "zip_data",
	} {
		out := "run: deferred after " + which + "\nmodule: deferred\n"
		cases = append(cases, exitCase{name: "callbacks", args: []string{which}, status: 11, stdout: &out})
	}
	for _, which := range []string{"exec", "exec_timeout", "exec_stdin", "exec_start"} {
		out := "run: deferred after " + which + "\nmodule: deferred\n"
		cases = append(cases, exitCase{name: "callbacks", args: []string{which}, status: 31, stdout: &out, shell: true})
	}
	// A show that exits where no error can be returned ends the process at
	// once: no deferred call runs.
	empty := ""
	cases = append(cases, exitCase{name: "callbacks", args: []string{"error_message"}, status: 11, stdout: &empty})

	// Executables are built on first use and kept for the whole test, so
	// the callback cases share one build.
	exeDir := t.TempDir()
	executables := map[string]string{}
	executable := func(t *testing.T, name string, race bool) string {
		t.Helper()
		key := name
		if race {
			key += "/race"
		}
		if exe, ok := executables[key]; ok {
			return exe
		}
		file := filepath.Join(exitDir, name+".goblin")
		mod, err := source.LoadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var buildLog bytes.Buffer
		exe, err := transpiler.BuildExecutable(mod, file, transpiler.BuildOptions{
			Output: filepath.Join(exeDir, strings.ReplaceAll(key, "/", "-")),
			Race:   race,
			Stdout: &buildLog,
			Stderr: &buildLog,
		})
		if err != nil {
			t.Fatalf("%v\n%s", err, buildLog.String())
		}
		executables[key] = exe
		return exe
	}

	for _, c := range cases {
		label := strings.Join(append([]string{c.name}, c.args...), "/")
		if c.shell && runtime.GOOS == "windows" {
			continue
		}
		backends := []struct {
			name    string
			command func(t *testing.T) *exec.Cmd
		}{
			{"interpreter", func(t *testing.T) *exec.Cmd {
				return exec.Command(goblin, append([]string{"run", filepath.Join(exitDir, c.name+".goblin")}, c.args...)...)
			}},
			{"compiled", func(t *testing.T) *exec.Cmd {
				return exec.Command(executable(t, c.name, false), c.args...)
			}},
		}
		if c.race {
			backends = append(backends, struct {
				name    string
				command func(t *testing.T) *exec.Cmd
			}{"race", func(t *testing.T) *exec.Cmd {
				return exec.Command(executable(t, c.name, true), c.args...)
			}})
		}
		for _, b := range backends {
			t.Run(b.name+"/"+label, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				cmd := b.command(t)
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				status := 0
				if err := cmd.Run(); err != nil {
					var exit *exec.ExitError
					if !errors.As(err, &exit) {
						t.Fatal(err)
					}
					status = exit.ExitCode()
				}
				if status != c.status {
					t.Errorf("exit status %d, want %d\nstderr: %s", status, c.status, stderr.String())
				}
				if c.stdout != nil {
					if got := normalize(stdout.String()); got != *c.stdout {
						t.Errorf("stdout:\n%q\nwant:\n%q", got, *c.stdout)
					}
					if got := normalize(stderr.String()); got != "" {
						t.Errorf("stderr:\n%q\nwant it empty", got)
					}
					return
				}
				checkOutput(t, exitDir, c.name, ".stdout", stdout.String(), true)
				checkOutput(t, exitDir, c.name, ".stderr", stderr.String(), false)
			})
		}
	}
}
