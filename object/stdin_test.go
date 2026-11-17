package object

import (
	"bufio"
	"strings"
	"testing"
)

// ReadLine splits a stream exactly as String.lines() splits text: only "\n"
// and "\r\n" end a line, so a lone "\r" stays in the line, last line included.
func TestReadLineAgreesWithLines(t *testing.T) {
	for _, text := range []string{"a\nb\n", "a\r\nb", "a\rb\n\nc\r\n", "\n", "", "a\r", "a\r\r\n", "x\ny\r"} {
		r := bufio.NewReader(strings.NewReader(text))
		var got []string
		for {
			line, ok, err := ReadLine(r)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				break
			}
			got = append(got, line)
		}
		want, err := String(text).Lines(CallArgs{})
		if err != nil {
			t.Fatal(err)
		}
		var wantLines []string
		for _, e := range want.(*List).Elements {
			wantLines = append(wantLines, string(e.(String)))
		}
		if strings.Join(got, "|") != strings.Join(wantLines, "|") || len(got) != len(wantLines) {
			t.Errorf("ReadLine(%q) = %q, lines() = %q", text, got, wantLines)
		}
	}
}
