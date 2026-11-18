package os

import (
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/object"
)

func load(t *testing.T, argv ...string) *object.Module {
	t.Helper()
	m, err := ExecuteWithFrozenArgs(argv)
	if err != nil {
		t.Fatal(err)
	}
	return m.(*object.Module)
}

func callMethod(t *testing.T, v object.Object, name string, a object.CallArgs) (object.Object, error) {
	t.Helper()
	attr, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return attr.(*object.Function).Call(a)
}

func TestArgsAndProgram(t *testing.T) {
	m := load(t, "app.goblin", "a", "b")
	args := m.Members["args"].(*object.List)
	if len(args.Elements) != 2 || args.Elements[0] != object.String("a") || args.Elements[1] != object.String("b") {
		t.Fatalf("args = %v", args)
	}
	if m.Members["program"] != object.String("app.goblin") {
		t.Fatalf("program = %v", m.Members["program"])
	}
	if m.Members["args"] != object.Object(args) {
		t.Fatal("args is not one list")
	}

	repl := load(t, "")
	if l := repl.Members["args"].(*object.List); len(l.Elements) != 0 || repl.Members["program"] != object.String("") {
		t.Fatalf("REPL args/program = %v / %v", l, repl.Members["program"])
	}

	compiled, err := Execute()
	if err != nil {
		t.Fatal(err)
	}
	cm := compiled.(*object.Module)
	if cm.Members["program"] != object.String(os.Args[0]) || len(cm.Members["args"].(*object.List).Elements) != len(os.Args)-1 {
		t.Fatal("Execute() does not split os.Args into program and args")
	}
}

func TestValues(t *testing.T) {
	m := load(t, "x")
	if m.Members["pid"] != object.Integer(os.Getpid()) {
		t.Fatal("pid")
	}
	if m.Members["platform"] != object.String(runtime.GOOS) {
		t.Fatal("platform")
	}
	for _, name := range []string{"stdin", "stdout", "stderr"} {
		f, ok := m.Members[name].(*filestream.File)
		if !ok {
			t.Fatalf("%s is %T", name, m.Members[name])
		}
		if p, _ := f.GetAttr("path"); p != object.Nil {
			t.Fatalf("%s.path = %v", name, p)
		}
	}
	if load(t, "y").Members["stdin"] != m.Members["stdin"] {
		t.Fatal("two loads have different stdin values")
	}
	if _, err := callMethod(t, m.Members["stdout"], "read", object.CallArgs{}); !errors.Is(err, object.ValueError) {
		t.Fatalf("stdout.read() error = %v", err)
	}
	if _, err := callMethod(t, m.Members["stdin"], "write", object.CallArgs{Positional: object.Args{object.String("x")}}); !errors.Is(err, object.ValueError) {
		t.Fatalf("stdin.write() error = %v", err)
	}
	if _, err := m.Members["exit"].(*object.Function).Call(object.CallArgs{Positional: object.Args{object.Integer(256)}}); !errors.Is(err, object.ValueError) {
		t.Fatalf("exit(256) error = %v", err)
	}
	// exit raises the exit request the top of the task ends the process
	// with, after the deferred calls on the way have run.
	exit := m.Members["exit"].(*object.Function)
	if _, err := exit.Call(object.CallArgs{Positional: object.Args{object.Integer(3)}}); err == nil {
		t.Fatal("exit(3) returned")
	} else if code, ok := object.ExitCode(err); !ok || code != 3 {
		t.Fatalf("exit(3) error = %v", err)
	}
	if _, err := exit.Call(object.CallArgs{}); err == nil {
		t.Fatal("exit() returned")
	} else if code, ok := object.ExitCode(err); !ok || code != 0 {
		t.Fatalf("exit() error = %v", err)
	}
	if _, err := m.Members["Env"].(*object.Function).Call(object.CallArgs{}); !errors.Is(err, object.TypeError) {
		t.Fatalf("Env() error = %v", err)
	}
}

func TestEnv(t *testing.T) {
	const name = "GOBLIN_OS_TEST_VARIABLE"
	t.Setenv(name, "")
	os.Unsetenv(name)
	env := load(t, "x").Members["env"]

	_, err := env.Index(object.String(name))
	var e *object.Error
	if !errors.As(err, &e) || !errors.Is(err, object.KeyError) || e.Value != "environment variable '"+name+"' is not set" {
		t.Fatalf("env[missing] error = %v", err)
	}
	if v, _ := callMethod(t, env, "get", object.CallArgs{Positional: object.Args{object.String(name)}}); v != object.Nil {
		t.Fatalf("get(missing) = %v", v)
	}
	if v, _ := callMethod(t, env, "get", object.CallArgs{Positional: object.Args{object.String(name)}, Keyword: object.Kwargs{{Name: "default", Value: object.String("d")}}}); v != object.String("d") {
		t.Fatalf("get(missing, default=) = %v", v)
	}

	if err := object.SetIndex(env, object.String(name), object.String("")); err != nil {
		t.Fatal(err)
	}
	if v := os.Getenv(name); v != "" {
		t.Fatalf("Setenv saw %q", v)
	}
	if v, _ := callMethod(t, env, "contains", object.CallArgs{Positional: object.Args{object.String(name)}}); v != object.True {
		t.Fatal("a variable set to \"\" is not contained")
	}
	object.SetIndex(env, object.String(name), object.String("v"))
	if v, err := env.Index(object.String(name)); err != nil || v != object.String("v") {
		t.Fatalf("env[name] = %v, %v", v, err)
	}
	d, _ := callMethod(t, env, "to_dict", object.CallArgs{})
	if v, err := d.Index(object.String(name)); err != nil || v != object.String("v") {
		t.Fatalf("to_dict()[name] = %v, %v", v, err)
	}
	keys := d.(*object.Dict).Entries()
	for i := 1; i < len(keys); i++ {
		if string(keys[i-1].Key.(object.String)) > string(keys[i].Key.(object.String)) {
			t.Fatal("to_dict() is not sorted by name")
		}
	}

	if r, _ := callMethod(t, env, "remove", object.CallArgs{Positional: object.Args{object.String(name)}}); r != env {
		t.Fatal("remove() does not return the Env")
	}
	if _, ok := os.LookupEnv(name); ok {
		t.Fatal("remove() left the variable")
	}
	if _, err := callMethod(t, env, "remove", object.CallArgs{Positional: object.Args{object.String(name)}}); err != nil {
		t.Fatalf("remove(absent) error = %v", err)
	}

	for _, bad := range []string{"", "A=B", "A\x00"} {
		if err := object.SetIndex(env, object.String(bad), object.String("x")); !errors.Is(err, object.ValueError) {
			t.Errorf("env[%q] = x error = %v, want ValueError", bad, err)
		}
	}
	if err := object.SetIndex(env, object.String(name), object.Integer(1)); !errors.Is(err, object.TypeError) {
		t.Fatalf("env[name] = 1 error = %v", err)
	}
	if _, err := env.Index(object.Integer(1)); !errors.Is(err, object.TypeError) {
		t.Fatalf("env[1] error = %v", err)
	}
	if _, err := object.Collect(env); err == nil {
		t.Fatal("Env is iterable")
	}
}

// os.stdin reads through object.Stdin, the buffer input() reads, so the two
// can be mixed without losing or reordering lines.
func TestStdinSharesTheInputBuffer(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig; r.Close() }()
	if _, err := w.WriteString("one\ntwo\r\nthree\nrest"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	stdin := load(t, "").Members["stdin"]
	next := func() object.Object {
		line, ok, err := object.ReadLine(object.Stdin) // what input() reads
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return object.Nil
		}
		return object.String(line)
	}
	if got := next(); got != object.String("one") {
		t.Fatalf("first line = %v", got)
	}
	if got, err := callMethod(t, stdin, "read_line", object.CallArgs{}); err != nil || got != object.String("two") {
		t.Fatalf("read_line() = %v, %v", got, err)
	}
	if got := next(); got != object.String("three") {
		t.Fatalf("third line = %v", got)
	}
	got, err := callMethod(t, stdin, "read", object.CallArgs{})
	if err != nil || string(got.(object.Bytes)) != "rest" {
		t.Fatalf("read() = %v, %v", got, err)
	}
}
