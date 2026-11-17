package object

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// Stdin is the process's standard input behind the one buffer every reader
// of it shares: input() and the os module's stdin stream both read through
// it, so mixing them never loses or reorders data. It reads whatever
// os.Stdin is when the read happens, as print writes to whatever os.Stdout
// is. It lives here rather than in extension/ because the builtins, which
// every compiled program links, read it.
var Stdin = bufio.NewReader(stdinFile{})

type stdinFile struct{}

func (stdinFile) Read(p []byte) (int, error) { return os.Stdin.Read(p) }

// ReadLine reads the next line from r without its terminator ("\n" or
// "\r\n"); a last line with no terminator counts. ok is false at the end of
// the stream. It does not check the text is UTF-8: callers report that in
// their own words.
func ReadLine(r *bufio.Reader) (line string, ok bool, err error) {
	text, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, err
	}
	if err == io.EOF && text == "" {
		return "", false, nil
	}
	// Only "\n" and "\r\n" end a line, as String.lines() has it: a lone
	// "\r" at the end of the stream is part of the last line.
	if strings.HasSuffix(text, "\n") {
		text = strings.TrimSuffix(text[:len(text)-1], "\r")
	}
	return text, true, nil
}
