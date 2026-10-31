package transpiler

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aisk/goblin/ast"
)

// BuildOptions configures BuildExecutable.
type BuildOptions struct {
	// Output is the executable path. Empty means the source file's base name
	// without the .goblin extension; a relative path resolves against the
	// working directory.
	Output string
	// Race builds with the Go race detector.
	Race bool
	// Verbose logs the build directory and the go command to Stderr.
	Verbose bool
	// Stdout and Stderr receive the go command's output. Nil discards it.
	Stdout, Stderr io.Writer
}

// BuildExecutable transpiles a checked module (see source.LoadFile) read from
// sourceFile and compiles it with the go tool. It returns the absolute path
// of the executable.
func BuildExecutable(mod *ast.Module, sourceFile string, opts BuildOptions) (string, error) {
	out := opts.Output
	if out == "" {
		out = strings.TrimSuffix(filepath.Base(sourceFile), ".goblin")
	}
	// go build runs in the temporary directory, where a relative -o would
	// land (and be removed along with it).
	out, err := filepath.Abs(out)
	if err != nil {
		return "", fmt.Errorf("failed to resolve output path: %w", err)
	}

	dir, err := os.MkdirTemp("", "goblin-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	if err := TranspileToDir(mod, sourceFile, dir); err != nil {
		return "", err
	}

	args := []string{"build", "-mod=mod"}
	if opts.Race {
		args = append(args, "-race")
	}
	args = append(args, "-o", out, ".")
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if opts.Verbose && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "build dir: %s\n", dir)
		fmt.Fprintf(opts.Stderr, "run: (cd %s && %s)\n", dir, strings.Join(cmd.Args, " "))
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build failed: %w", err)
	}
	return out, nil
}
