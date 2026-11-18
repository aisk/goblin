package exec

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

// TestHelperProcess is the program the tests run: the test binary itself,
// told what to do by the arguments after "--".
func TestHelperProcess(t *testing.T) {
	args, helping := []string(nil), false
	for i, arg := range os.Args {
		if arg == "--" {
			args, helping = os.Args[i+1:], true
			break
		}
	}
	if os.Getenv("GOBLIN_EXEC_HELPER") != "1" || !helping {
		return
	}
	for _, step := range args {
		verb, value, _ := strings.Cut(step, "=")
		switch verb {
		case "out":
			os.Stdout.WriteString(value)
		case "err":
			os.Stderr.WriteString(value)
		case "bad-utf8":
			os.Stdout.Write([]byte("a\xffb"))
		case "bad-utf8-err":
			os.Stderr.Write([]byte("a\xffb"))
		case "cat":
			io.Copy(os.Stdout, os.Stdin)
		case "env":
			os.Stdout.WriteString(os.Getenv(value) + "|")
		case "has-env":
			_, ok := os.LookupEnv(value)
			os.Stdout.WriteString(strconv.FormatBool(ok) + "|")
		case "cwd":
			wd, _ := os.Getwd()
			os.Stdout.WriteString(wd)
		case "sleep":
			d, _ := time.ParseDuration(value)
			time.Sleep(d)
		case "exit":
			code, _ := strconv.Atoi(value)
			os.Exit(code)
		}
	}
	os.Exit(0)
}

// helper is the args List that runs TestHelperProcess with steps.
func helper(steps ...string) *object.List {
	elems := []object.Object{object.String(os.Args[0]), object.String("-test.run=^TestHelperProcess$"), object.String("--")}
	for _, s := range steps {
		elems = append(elems, object.String(s))
	}
	return &object.List{Elements: elems}
}

func TestMain(m *testing.M) {
	os.Setenv("GOBLIN_EXEC_HELPER", "1")
	os.Exit(m.Run())
}

func kw(name string, v object.Object) object.KwArg { return object.KwArg{Name: name, Value: v} }

func call(t *testing.T, name string, list *object.List, kws ...object.KwArg) object.Object {
	t.Helper()
	return modtest.CallModule(t, Execute, name, object.CallArgs{Positional: object.Args{list}, Keyword: kws})
}

func callErr(name string, list object.Object, kws ...object.KwArg) error {
	m, _ := Execute()
	_, err := m.(*object.Module).Members[name].(*object.Function).Call(object.CallArgs{Positional: object.Args{list}, Keyword: kws})
	return err
}

func get(t *testing.T, v object.Object, name string) object.Object {
	t.Helper()
	got, err := v.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func method(t *testing.T, v object.Object, name string, positional ...object.Object) (object.Object, error) {
	t.Helper()
	return get(t, v, name).(*object.Function).Call(object.CallArgs{Positional: positional})
}

func TestCapture(t *testing.T) {
	r := call(t, "capture", helper("out=hello\n", "err=warn\n"))
	if get(t, r, "stdout") != object.String("hello\n") || get(t, r, "stderr") != object.String("warn\n") ||
		get(t, r, "code") != object.Integer(0) || get(t, r, "success") != object.True {
		t.Fatalf("capture() = %v %v %v", get(t, r, "stdout"), get(t, r, "stderr"), get(t, r, "code"))
	}
	// Output that is not text is refused rather than altered, after the
	// exit status has been checked; the ExitError's copy is diagnostic and
	// keeps a replacement character instead.
	for _, stream := range []string{"stdout", "stderr"} {
		step := "bad-utf8"
		if stream == "stderr" {
			step = "bad-utf8-err"
		}
		err := callErr("capture", helper(step), kw("check", object.False))
		want := `" wrote ` + stream + ` that is not valid UTF-8 (use exec.start with ` + stream + `=exec.PIPE for bytes)`
		if !errors.Is(err, object.ValueError) || !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("invalid UTF-8 on %s = %v", stream, err)
		}
	}
	err := callErr("capture", helper("bad-utf8", "exit=2"))
	var exitErr *object.Error
	if !errors.As(err, &exitErr) || !errors.Is(err, ExitError) {
		t.Fatalf("invalid UTF-8 with a failing exit = %v", err)
	}
	if out, _ := exitErr.Field("stdout"); out != object.String("a\uFFFDb") {
		t.Fatalf("ExitError stdout = %q", out)
	}
	r = call(t, "capture", helper("cat"), kw("stdin", object.String("fed")))
	if get(t, r, "stdout") != object.String("fed") {
		t.Fatalf("stdin= String fed %q", get(t, r, "stdout"))
	}
	r = call(t, "capture", helper("cat"), kw("stdin", object.Bytes("raw")))
	if get(t, r, "stdout") != object.String("raw") {
		t.Fatalf("stdin= Bytes fed %q", get(t, r, "stdout"))
	}

	err = callErr("capture", helper("err=first\n", "err=fatal: not a repo\n\n", "exit=128"))
	if err == nil || !strings.HasSuffix(err.Error(), `exited with status 128: fatal: not a repo`) || !strings.HasPrefix(err.Error(), `capture() "`) {
		t.Fatalf("capture() failure error = %v", err)
	}
	r = call(t, "capture", helper("exit=3"), kw("check", object.False))
	if get(t, r, "code") != object.Integer(3) || get(t, r, "success") != object.False {
		t.Fatal("check=false result")
	}
}

func TestRun(t *testing.T) {
	var out, errs bytes.Buffer
	r := call(t, "run", helper("out=o", "err=e"), kw("stdout", modtest.DestRecorder(&out)), kw("stderr", modtest.DestRecorder(&errs)))
	if out.String() != "o" || errs.String() != "e" {
		t.Fatalf("run() wrote %q / %q", out.String(), errs.String())
	}
	if get(t, r, "stdout") != object.Nil || get(t, r, "stderr") != object.Nil {
		t.Fatal("run() result has output")
	}
	var both bytes.Buffer
	shared := modtest.DestRecorder(&both)
	call(t, "run", helper("out=1", "err=2"), kw("stdout", shared), kw("stderr", shared))
	if both.Len() != 2 {
		t.Fatalf("shared writer got %q", both.String())
	}

	err := callErr("run", helper("exit=2"))
	var e *object.Error
	if !errors.As(err, &e) || !strings.HasSuffix(e.Value, "exited with status 2") || errors.Is(err, object.IOError) {
		t.Fatalf("run() failure error = %v", err)
	}

	missing := &object.List{Elements: []object.Object{object.String("goblin-no-such-program")}}
	err = callErr("run", missing)
	if !errors.Is(err, object.NotExistError) || err.Error() != `run() program "goblin-no-such-program" not found` {
		t.Fatalf("missing program error = %v", err)
	}
	empty := &object.List{Elements: []object.Object{object.String("")}}
	if err := callErr("capture", empty); !errors.Is(err, object.ValueError) || err.Error() != `capture() argument 'args' has an empty program name` {
		t.Fatalf("empty program error = %v", err)
	}
	// A limit that has passed before the start is the ordinary timeout.
	for _, fn := range []string{"run", "capture"} {
		err = callErr(fn, helper("sleep=10s"), kw("timeout", object.Integer(0)))
		if !errors.Is(err, object.TimeoutError) || err.Error() != fn+`() "`+os.Args[0]+`" timed out after 0 seconds` {
			t.Fatalf("%s(timeout=0) = %v", fn, err)
		}
		if !errors.As(err, &e) {
			t.Fatal(err)
		}
		if _, ok := e.Field("stdout"); !ok {
			t.Fatalf("%s(timeout=0) carries no stdout", fn)
		}
	}

	start := time.Now()
	err = callErr("run", helper("sleep=10s"), kw("timeout", object.Float(0.2)))
	if !errors.Is(err, object.TimeoutError) || !strings.Contains(err.Error(), "timed out after 0.2 seconds") {
		t.Fatalf("timeout error = %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the timed-out command was not killed")
	}

	for _, bad := range []struct {
		list object.Object
		kws  []object.KwArg
		kind *object.Error
	}{
		{&object.List{}, nil, object.ValueError},
		{&object.List{Elements: []object.Object{object.Integer(1)}}, nil, object.TypeError},
		{helper(), []object.KwArg{kw("stdout", pipe)}, object.TypeError},
		{helper(), []object.KwArg{kw("stdin", pipe)}, object.TypeError},
		{helper(), []object.KwArg{kw("stdout", object.Integer(1))}, object.TypeError},
		{helper(), []object.KwArg{kw("env", &object.List{})}, object.TypeError},
		{helper(), []object.KwArg{kw("timeout", object.Integer(-1))}, object.ValueError},
	} {
		if err := callErr("run", bad.list, bad.kws...); !errors.Is(err, bad.kind) {
			t.Errorf("run(%v, %v) error = %v, want %s", bad.list, bad.kws, err, bad.kind)
		}
	}
}

func TestEnvAndCwd(t *testing.T) {
	t.Setenv("GOBLIN_EXEC_INHERITED", "kept")
	t.Setenv("GOBLIN_EXEC_DROPPED", "x")
	env := object.NewDict()
	env.Set(object.String("GOBLIN_EXEC_ADDED"), object.String("added"))
	env.Set(object.String("GOBLIN_EXEC_DROPPED"), object.Nil)
	r := call(t, "capture", helper("env=GOBLIN_EXEC_INHERITED", "env=GOBLIN_EXEC_ADDED", "has-env=GOBLIN_EXEC_DROPPED"), kw("env", env))
	if got := get(t, r, "stdout"); got != object.String("kept|added|false|") {
		t.Fatalf("env = %q", got)
	}
	// Names and values are checked as os.env checks them.
	for _, c := range []struct {
		name  string
		value object.Object
		msg   string
	}{
		{"A=B", object.String("c"), `capture() invalid environment variable name "A=B"`},
		{"", object.String("c"), `capture() invalid environment variable name ""`},
		{"", object.Nil, `capture() invalid environment variable name ""`},
		{"Z", object.String("b\x00c"), `capture() environment variable 'Z' value contains NUL`},
	} {
		bad := object.NewDict()
		bad.Set(object.String(c.name), c.value)
		err := callErr("capture", helper(), kw("env", bad))
		if !errors.Is(err, object.ValueError) || err.Error() != c.msg {
			t.Errorf("env %q = %v, want %q", c.name, err, c.msg)
		}
	}

	dir := t.TempDir()
	r = call(t, "capture", helper("cwd"), kw("cwd", pathvalue.NewPath(dir)))
	got, _ := filepath.EvalSymlinks(string(get(t, r, "stdout").(object.String)))
	want, _ := filepath.EvalSymlinks(dir)
	if got != want {
		t.Fatalf("cwd = %q, want %q", got, want)
	}
	missing := filepath.Join(dir, "missing")
	if err := callErr("run", helper(), kw("cwd", object.String(missing))); !errors.Is(err, object.NotExistError) || err.Error() != `run() working directory "`+missing+`": no such file or directory` {
		t.Fatalf("missing cwd error = %v", err)
	}
	// A file where the directory belongs is an IOError, as fs reports it.
	if err := callErr("run", helper(), kw("cwd", object.String(os.Args[0]))); !errors.Is(err, object.IOError) || errors.Is(err, object.NotExistError) || !strings.HasSuffix(err.Error(), `" is not a directory`) {
		t.Fatalf("file cwd error = %v", err)
	}
}

func TestStart(t *testing.T) {
	c := call(t, "start", helper("out=one\nREADY\n", "sleep=10s"), kw("stdout", pipe))
	if get(t, c, "running") != object.True || get(t, c, "stdin") != object.Nil {
		t.Fatal("a started command is not running")
	}
	if pid, ok := get(t, c, "pid").(object.Integer); !ok || pid <= 0 {
		t.Fatal("pid")
	}
	lines, err := object.Iterate(get(t, c, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	for {
		line, ok, err := lines.Next()
		if err != nil || !ok {
			t.Fatalf("pipe ended before READY: %v", err)
		}
		if line == object.String("READY") {
			break
		}
	}
	lines.Close()
	if _, err := method(t, c, "wait", object.Float(0.05)); !errors.Is(err, object.TimeoutError) {
		t.Fatalf("wait(timeout) error = %v", err)
	}
	if get(t, c, "running") != object.True {
		t.Fatal("a wait timeout stopped the process")
	}
	if _, err := method(t, c, "kill"); err != nil {
		t.Fatal(err)
	}
	// A killed process has status -1, which wait() raises as a failure;
	// check=false returns the Result, the same one every time.
	_, err = method(t, c, "wait")
	var exitErr *object.Error
	if !errors.As(err, &exitErr) || !errors.Is(err, ExitError) || !strings.HasPrefix(err.Error(), `wait() "`) || !strings.HasSuffix(err.Error(), `exited with status -1`) {
		t.Fatalf("wait() after kill = %v", err)
	}
	if code, _ := exitErr.Field("code"); code != object.Integer(-1) {
		t.Fatalf("ExitError code = %v", code)
	}
	if out, _ := exitErr.Field("stdout"); out != object.Nil {
		t.Fatalf("ExitError stdout = %v", out)
	}
	if _, again := method(t, c, "wait"); again == nil || again.Error() != err.Error() {
		t.Fatalf("wait() again = %v", again)
	}
	r, err := get(t, c, "wait").(*object.Function).Call(object.CallArgs{Keyword: object.Kwargs{kw("check", object.False)}})
	if err != nil {
		t.Fatal(err)
	}
	if get(t, r, "code") != object.Integer(-1) || get(t, c, "running") != object.False {
		t.Fatalf("killed command code = %v", get(t, r, "code"))
	}
	if again, _ := get(t, c, "wait").(*object.Function).Call(object.CallArgs{Keyword: object.Kwargs{kw("check", object.False)}}); again != r {
		t.Fatal("wait(check=false) again returned another Result")
	}
	if _, err := method(t, c, "kill"); err != nil {
		t.Fatalf("kill() after exit error = %v", err)
	}
	method(t, get(t, c, "stdout"), "close")

	cat := call(t, "start", helper("cat", "err=done"), kw("stdin", pipe), kw("stdout", pipe), kw("stderr", pipe))
	method(t, get(t, cat, "stdin"), "write", object.String("fed\n"))
	method(t, get(t, cat, "stdin"), "close")
	out, _ := method(t, get(t, cat, "stdout"), "read")
	errOut, _ := method(t, get(t, cat, "stderr"), "read")
	if !modtest.Equal(out, object.Bytes("fed\n")) || !modtest.Equal(errOut, object.Bytes("done")) {
		t.Fatalf("pipes carried %v / %v", out, errOut)
	}
	r, _ = method(t, cat, "wait")
	if get(t, r, "success") != object.True || get(t, r, "stdout") != object.Nil {
		t.Fatal("wait() result")
	}

	if !strings.HasPrefix(cat.(*Command).String(), `<Command ["`) {
		t.Fatalf("Show = %s", cat)
	}
	if err := callErr("start", helper(), kw("check", object.True)); !errors.Is(err, object.TypeError) {
		t.Fatal("start() accepted check=")
	}
}

func TestFindAndTypes(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m, _ := Execute()
	find := m.(*object.Module).Members["find"].(*object.Function)
	got, err := find.Call(object.CallArgs{Positional: object.Args{object.String(self)}})
	if err != nil || got.(*pathvalue.Path).FilePath() != filepath.Clean(self) {
		t.Fatalf("find(self) = %v, %v", got, err)
	}
	t.Setenv("PATH", filepath.Dir(self))
	got, _ = find.Call(object.CallArgs{Positional: object.Args{object.String(filepath.Base(self))}})
	if got == object.Nil {
		t.Fatal("find() did not search PATH")
	}
	if got, _ := find.Call(object.CallArgs{Positional: object.Args{object.String("goblin-no-such-program")}}); got != object.Nil {
		t.Fatalf("find(missing) = %v", got)
	}
	for name, hint := range map[string]string{"Command": "exec.start(args)", "Result": "exec.run(args)"} {
		_, err := m.(*object.Module).Members[name].(*object.Function).Call(object.CallArgs{})
		if !errors.Is(err, object.TypeError) || !strings.Contains(err.Error(), hint) {
			t.Errorf("%s() error = %v", name, err)
		}
	}
	if _, exported := m.(*object.Module).Members["Pipe"]; exported {
		t.Fatal("the type of PIPE is exported")
	}
}

func field(t *testing.T, err error, name string) object.Object {
	t.Helper()
	var e *object.Error
	if !errors.As(err, &e) {
		t.Fatalf("error %v is not a Goblin error", err)
	}
	v, ok := e.Field(name)
	if !ok {
		t.Fatalf("error %q has no data %q", err, name)
	}
	return v
}

func TestExitErrorData(t *testing.T) {
	err := callErr("run", helper("exit=3"))
	if !errors.Is(err, ExitError) || errors.Is(err, object.IOError) {
		t.Fatalf("run() exit error = %v", err)
	}
	if field(t, err, "code") != object.Integer(3) || field(t, err, "stdout") != object.Nil || field(t, err, "stderr") != object.Nil {
		t.Fatal("run() ExitError data")
	}

	err = callErr("capture", helper("out=out\n", "err=err\n", "exit=2"))
	if !errors.Is(err, ExitError) || field(t, err, "code") != object.Integer(2) ||
		field(t, err, "stdout") != object.String("out\n") || field(t, err, "stderr") != object.String("err\n") {
		t.Fatalf("capture() ExitError = %v", err)
	}
	if !strings.HasSuffix(err.Error(), "exited with status 2: err") {
		t.Fatalf("capture() message = %q", err)
	}

	killed := &object.List{Elements: []object.Object{object.String("sh"), object.String("-c"), object.String("kill -9 $$")}}
	err = callErr("run", killed)
	if !errors.Is(err, ExitError) || field(t, err, "code") != object.Integer(-1) {
		t.Fatalf("killed error = %v", err)
	}

	err = callErr("capture", helper("out=partial\n", "sleep=10s"), kw("timeout", object.Float(0.3)))
	if !errors.Is(err, object.TimeoutError) || field(t, err, "stdout") != object.String("partial\n") || field(t, err, "stderr") != object.String("") {
		t.Fatalf("capture() timeout = %v", err)
	}
	err = callErr("run", helper("sleep=10s"), kw("timeout", object.Float(0.1)))
	if field(t, err, "stdout") != object.Nil || field(t, err, "stderr") != object.Nil {
		t.Fatal("run() timeout data")
	}

	dir := t.TempDir()
	for _, cwd := range []string{filepath.Join(dir, "missing"), os.Args[0]} {
		err = callErr("run", helper(), kw("cwd", object.String(cwd)))
		if p := field(t, err, "path"); p.(*pathvalue.Path).FilePath() != cwd {
			t.Fatalf("cwd error path = %v", p)
		}
	}
}

// exiter is a stream whose read and write raise the exit request os.exit
// raises.
func exiter(code int) object.Object {
	fail := func(object.CallArgs) (object.Object, error) { return nil, object.NewExit(code) }
	return &object.Module{Name: "exiter", Members: map[string]object.Object{
		"write": &object.Function{Name: "write", Fn: fail},
		"read":  &object.Function{Name: "read", Fn: fail},
	}}
}

// An exit request a stream raises is returned ahead of what the process
// did: os/exec drops a stream's error when the child fails or times out.
func TestStreamExit(t *testing.T) {
	wantExit := func(what string, err error) {
		t.Helper()
		if code, ok := object.ExitCode(err); !ok || code != 3 {
			t.Errorf("%s error = %v, want exit 3", what, err)
		}
	}
	wantExit("run(stdout=) of a failing child", callErr("run", helper("out=hi", "exit=1"), kw("stdout", exiter(3))))
	wantExit("run(stderr=) of a failing child", callErr("run", helper("err=hi", "exit=1"), kw("stderr", exiter(3))))
	wantExit("run(stdout=) under a timeout", callErr("run", helper("out=hi", "sleep=10s"), kw("stdout", exiter(3)), kw("timeout", object.Float(0.2))))
	wantExit("run(stdin=) of a failing child", callErr("run", helper("cat", "exit=1"), kw("stdin", exiter(3))))
	wantExit("capture(stdin=) of a failing child", callErr("capture", helper("cat", "exit=1"), kw("stdin", exiter(3))))

	for _, stream := range []string{"stdout", "stdin"} {
		steps := []string{"out=hi", "exit=1"}
		if stream == "stdin" {
			steps = []string{"cat", "exit=1"}
		}
		m, _ := Execute()
		c, err := m.(*object.Module).Members["start"].(*object.Function).Call(object.CallArgs{
			Positional: object.Args{helper(steps...)},
			Keyword:    object.Kwargs{kw(stream, exiter(3))},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = method(t, c, "wait")
		wantExit("start("+stream+"=).wait()", err)
		// Waiting again gives the same outcome.
		_, err = method(t, c, "wait")
		wantExit("start("+stream+"=).wait() again", err)
	}

	// An ordinary error from a stream is left to os/exec as before.
	plain := &object.Module{Name: "plain", Members: map[string]object.Object{
		"write": &object.Function{Name: "write", Fn: func(object.CallArgs) (object.Object, error) {
			return nil, object.NewValueError("cannot write")
		}},
	}}
	if err := callErr("run", helper("out=hi", "exit=1"), kw("stdout", plain)); object.IsExit(err) || !strings.Contains(err.Error(), "exited with status 1") {
		t.Errorf("run() with a failing writer error = %v", err)
	}
}
