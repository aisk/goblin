package extension

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestEprintWritesToStderr(t *testing.T) {
	stdout, stderr := captureStdio(t, func() {
		_, err := Eprint(object.CallArgs{
			Positional: []object.Object{object.String("warn"), object.Integer(7)},
		})
		if err != nil {
			t.Fatalf("eprint: %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "warn 7\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "warn 7\n")
	}
}

func TestEprintEmptyArgsWritesNewline(t *testing.T) {
	_, stderr := captureStdio(t, func() {
		_, err := Eprint(object.CallArgs{})
		if err != nil {
			t.Fatalf("eprint: %v", err)
		}
	})
	if stderr != "\n" {
		t.Fatalf("stderr = %q, want newline", stderr)
	}
}

func TestPrintWritesToStdout(t *testing.T) {
	stdout, stderr := captureStdio(t, func() {
		_, err := Print(object.CallArgs{
			Positional: []object.Object{object.String("hi"), object.Integer(1)},
		})
		if err != nil {
			t.Fatalf("print: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout != "hi 1\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "hi 1\n")
	}
}

func TestPrintAndEprintRejectKeywords(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func(object.CallArgs) (object.Object, error)
	}{
		{"print", Print},
		{"eprint", Eprint},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.fn(object.CallArgs{
				Positional: []object.Object{object.String("x")},
				Keyword:    object.Kwargs{{Name: "file", Value: object.String("stderr")}},
			})
			if err == nil {
				t.Fatal("expected error for keyword args")
			}
			if !strings.Contains(err.Error(), tc.name+"()") {
				t.Fatalf("error %q should name %s()", err, tc.name)
			}
		})
	}
}

func TestBuiltinsModuleRegistersEprint(t *testing.T) {
	fn, ok := BuiltinsModule.Members["eprint"].(*object.Function)
	if !ok || fn.Name != "eprint" {
		t.Fatalf("BuiltinsModule eprint = %#v", BuiltinsModule.Members["eprint"])
	}
}

func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		_ = outW.Close()
		_ = errW.Close()
		os.Stdout, os.Stderr = origOut, origErr
	}()

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, outR)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, errR)
		errCh <- buf.String()
	}()

	fn()

	_ = outW.Close()
	_ = errW.Close()
	return <-outCh, <-errCh
}

func TestRangeForms(t *testing.T) {
	kw := func(pairs ...any) object.Kwargs {
		var out object.Kwargs
		for i := 0; i < len(pairs); i += 2 {
			out = append(out, object.KwArg{Name: pairs[i].(string), Value: pairs[i+1].(object.Object)})
		}
		return out
	}
	tests := []struct {
		name string
		args object.CallArgs
		want string
	}{
		{"end only", object.CallArgs{Positional: object.Args{object.Integer(3)}}, "[0, 1, 2]"},
		{"start and end", object.CallArgs{Positional: object.Args{object.Integer(2), object.Integer(4)}}, "[2, 3]"},
		{"negative end", object.CallArgs{Positional: object.Args{object.Integer(-1)}}, "[]"},
		{"end keyword", object.CallArgs{Keyword: kw("end", object.Integer(2))}, "[0, 1]"},
		{"positional start, end keyword", object.CallArgs{Positional: object.Args{object.Integer(1)}, Keyword: kw("end", object.Integer(3))}, "[1, 2]"},
	}
	for _, test := range tests {
		got, err := Range(test.args)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if text, _ := got.ToString(); text != test.want {
			t.Errorf("%s: got %s, want %s", test.name, text, test.want)
		}
	}

	for _, args := range []object.CallArgs{
		{},
		{Keyword: kw("start", object.Integer(1))},
		{Positional: object.Args{object.String("3")}},
	} {
		if _, err := Range(args); err == nil {
			t.Errorf("range(%v) should fail", args)
		}
	}
}

// withStdin points object.Stdin at text for the duration of fn.
func withStdin(t *testing.T, text string, fn func()) {
	t.Helper()
	orig := object.Stdin
	object.Stdin = bufio.NewReader(strings.NewReader(text))
	defer func() { object.Stdin = orig }()
	fn()
}

func TestInputReadsLines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []object.Object
	}{
		{"newline", "a\nb\n", []object.Object{object.String("a"), object.String("b"), object.Nil}},
		{"crlf", "a\r\n\r\nb\r\n", []object.Object{object.String("a"), object.String(""), object.String("b"), object.Nil}},
		{"no terminator", "a\nlast", []object.Object{object.String("a"), object.String("last"), object.Nil}},
		{"empty input", "", []object.Object{object.Nil, object.Nil}},
		{"non-ASCII", "héllo\n", []object.Object{object.String("héllo"), object.Nil}},
	}
	for _, c := range cases {
		withStdin(t, c.input, func() {
			for i, want := range c.want {
				got, err := Input(object.CallArgs{})
				if err != nil || got != want {
					t.Errorf("%s: input() #%d = %v, %v; want %v", c.name, i, got, err, want)
				}
			}
		})
	}
}

func TestInputWritesThePrompt(t *testing.T) {
	var got object.Object
	stdout, _ := captureStdio(t, func() {
		withStdin(t, "Ada\n", func() {
			var err error
			got, err = Input(object.CallArgs{Positional: object.Args{object.String("Name: ")}})
			if err != nil {
				t.Fatal(err)
			}
		})
	})
	if stdout != "Name: " || got != object.String("Ada") {
		t.Fatalf("stdout %q, result %v", stdout, got)
	}
}

func TestInputErrors(t *testing.T) {
	withStdin(t, "\xff\n", func() {
		_, err := Input(object.CallArgs{})
		if err == nil || err.Error() != "input() line is not valid UTF-8" || !errors.Is(err, object.ValueError) {
			t.Errorf("invalid UTF-8: %v", err)
		}
	})
	_, err := Input(object.CallArgs{Positional: object.Args{object.Integer(1)}})
	if err == nil || err.Error() != "input() argument 'prompt' must be String, got Int" || !errors.Is(err, object.TypeError) {
		t.Errorf("non-String prompt: %v", err)
	}
	withStdin(t, "", func() {
		object.Stdin = bufio.NewReader(failingReader{})
		_, err := Input(object.CallArgs{})
		if err == nil || !strings.HasPrefix(err.Error(), "input() failed to read standard input: ") || !errors.Is(err, object.IOError) {
			t.Errorf("failed read: %v", err)
		}
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }
