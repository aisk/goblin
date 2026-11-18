package examples_test

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aisk/goblin/source"
	"github.com/aisk/goblin/transpiler"
)

// cliProgram is run with real arguments: cli.parse exits the process on
// --help and on a usage error, after the program's pending deferred calls
// run, which no in-process test can observe.
const cliProgram = "cliprocess/copy.goblin"

// TestCLIProcessMode runs the program under goblin run and as a compiled
// executable, checking the streams, the exit status and the program name
// each shows.
func TestCLIProcessMode(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the goblin binary and an executable")
	}
	goblin := filepath.Join(t.TempDir(), "goblin")
	build := exec.Command("go", "build", "-o", goblin, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	mod, err := source.LoadFile(cliProgram)
	if err != nil {
		t.Fatal(err)
	}
	var buildLog bytes.Buffer
	exe, err := transpiler.BuildExecutable(mod, cliProgram, transpiler.BuildOptions{
		Output: filepath.Join(t.TempDir(), "copy"),
		Stdout: &buildLog,
		Stderr: &buildLog,
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, buildLog.String())
	}

	backends := []struct {
		name    string
		program string // the name texts show
		command func(args ...string) *exec.Cmd
	}{
		{"interpreter", "copy.goblin", func(args ...string) *exec.Cmd {
			return exec.Command(goblin, append([]string{"run", cliProgram}, args...)...)
		}},
		{"compiled", filepath.Base(exe), func(args ...string) *exec.Cmd {
			return exec.Command(exe, args...)
		}},
	}
	for _, b := range backends {
		help := strings.Join([]string{
			"usage: " + b.program + " [options] <source> <target>",
			"",
			"Copy a file.",
			"",
			"options:",
			"  -v, --verbose  say more",
			"  -h, --help     show this help and exit",
			"",
		}, "\n")
		cases := []struct {
			name           string
			args           []string
			stdout, stderr string
			status         int
		}{
			{"good", []string{"a", "-v", "b"}, "Args(source=\"a\", target=\"b\", verbose=true)\ndeferred\n", "", 0},
			{"help", []string{"a", "--help"}, help + "deferred\n", "", 0},
			{"short help", []string{"-h"}, help + "deferred\n", "", 0},
			{"usage error", []string{"a"}, "deferred\n", "error: missing argument <target>\n" +
				"usage: " + b.program + " [options] <source> <target>\n" +
				"Run '" + b.program + " --help' for details.\n", 2},
			{"suggestion", []string{"a", "b", "--verbsoe"}, "deferred\n", "error: unknown option --verbsoe (did you mean --verbose?)\n" +
				"usage: " + b.program + " [options] <source> <target>\n" +
				"Run '" + b.program + " --help' for details.\n", 2},
		}
		for _, c := range cases {
			t.Run(b.name+"/"+c.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				cmd := b.command(c.args...)
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
					t.Errorf("exit status %d, want %d", status, c.status)
				}
				if got := normalize(stdout.String()); got != c.stdout {
					t.Errorf("stdout:\n%q\nwant:\n%q", got, c.stdout)
				}
				if got := normalize(stderr.String()); got != c.stderr {
					t.Errorf("stderr:\n%q\nwant:\n%q", got, c.stderr)
				}
			})
		}
	}
}
