// Package procargs splits a command line into the program and the
// arguments a Goblin program sees, for every module that reads them (os,
// cli), so they cannot disagree.
package procargs

import "os"

// Split separates argv into the program (argv[0], "" when argv is empty)
// and the arguments after it.
func Split(argv []string) (program string, args []string) {
	if len(argv) == 0 {
		return "", nil
	}
	return argv[0], argv[1:]
}

// Process is the running process's own command line: what a compiled
// program's modules read. The interpreter passes the script's instead, so a
// run never sees the goblin binary's own arguments.
func Process() []string { return os.Args }
