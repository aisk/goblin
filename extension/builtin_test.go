package extension

import (
	"bytes"
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
