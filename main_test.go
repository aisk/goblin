package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aisk/goblin/interpreter"
)

func completionStrings(c *replCompleter, line string) ([]string, int) {
	candidates, offset := c.Do([]rune(line), len([]rune(line)))
	result := make([]string, len(candidates))
	for i, candidate := range candidates {
		result[i] = string(candidate)
	}
	return result, offset
}

func TestREPLCompleter(t *testing.T) {
	s := interpreter.NewSession(".")
	if _, err := s.Eval(`type User(name) { func hello(self) { return self.name } }`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(`var user = User("alice")`); err != nil {
		t.Fatal(err)
	}
	c := &replCompleter{session: s}

	tests := []struct {
		line       string
		candidates []string
		offset     int
	}{
		{line: "pri", candidates: []string{"nt"}, offset: 3},
		{line: "ret", candidates: []string{"urn"}, offset: 3},
		{line: "user.na", candidates: []string{"me"}, offset: 2},
		{line: "print(user.he", candidates: []string{"llo"}, offset: 2},
		{line: "user.name.trim_s", candidates: []string{"uffix"}, offset: 6},
		{line: "make().pu", candidates: []string{}, offset: 0},
		{line: "missing.na", candidates: []string{}, offset: 2},
	}

	for _, tt := range tests {
		got, offset := completionStrings(c, tt.line)
		if !reflect.DeepEqual(got, tt.candidates) || offset != tt.offset {
			t.Errorf("complete(%q) = (%v, %d), want (%v, %d)", tt.line, got, offset, tt.candidates, tt.offset)
		}
	}
}

func TestCompletionPathAtCursor(t *testing.T) {
	path, prefix, ok := completionPath([]rune("user.na + value"), len([]rune("user.na")))
	if !ok || !reflect.DeepEqual(path, []string{"user"}) || prefix != "na" {
		t.Fatalf("completionPath = (%v, %q, %v)", path, prefix, ok)
	}
}

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
