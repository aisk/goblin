package exec

import (
	"os"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) > 0 {
		_, _ = os.Stdout.WriteString(args[0])
	}
	if len(args) > 1 {
		_, _ = os.Stderr.WriteString(args[1])
	}
	if len(args) > 2 && args[2] == "fail" {
		os.Exit(7)
	}
	os.Exit(0)
}

func helperCommand(t *testing.T, helperArgs ...string) *Cmd {
	t.Helper()
	argObjects := []object.Object{object.String("-test.run=TestHelperProcess"), object.String("--")}
	for _, arg := range helperArgs {
		argObjects = append(argObjects, object.String(arg))
	}
	obj, err := command(object.CallArgs{
		Positional: []object.Object{object.String(os.Args[0]), &object.List{Elements: argObjects}},
		Keyword: map[string]object.Object{
			"env": &object.Dict{Entries: map[string]object.DictEntry{
				"GO_WANT_HELPER_PROCESS": {Key: object.String("GO_WANT_HELPER_PROCESS"), Value: object.String("1")},
			}},
			"stdout": capture,
			"stderr": capture,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return obj.(*Cmd)
}

func TestRunCapturesOutputAndExitStatus(t *testing.T) {
	cmd := helperCommand(t, "out", "err", "fail")
	obj, err := cmd.run(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	result := obj.(*Result)
	if result.code != 7 {
		t.Fatalf("code = %d, want 7", result.code)
	}
	if result.Bool() {
		t.Fatal("non-zero result is truthy")
	}
	if got := string(result.stdout.(object.Bytes)); got != "out" {
		t.Fatalf("stdout = %q", got)
	}
	if got := string(result.stderr.(object.Bytes)); got != "err" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestStartWaitAndRepeatedWait(t *testing.T) {
	cmd := helperCommand(t, "ok")
	if _, err := cmd.start(object.CallArgs{}); err != nil {
		t.Fatal(err)
	}
	first, err := cmd.wait(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cmd.wait(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("repeated wait did not return cached result")
	}
	if _, err := cmd.start(object.CallArgs{}); err == nil {
		t.Fatal("repeated start succeeded")
	}
}

func TestWaitBeforeStartFails(t *testing.T) {
	cmd := helperCommand(t)
	if _, err := cmd.wait(object.CallArgs{}); err == nil {
		t.Fatal("wait before start succeeded")
	}
}

func TestFailedStartCannotBeRetried(t *testing.T) {
	obj, err := command(object.CallArgs{Positional: []object.Object{object.String("/definitely/not/a/goblin-command")}})
	if err != nil {
		t.Fatal(err)
	}
	cmd := obj.(*Cmd)
	if _, err := cmd.start(object.CallArgs{}); err == nil {
		t.Fatal("missing command started")
	}
	if _, err := cmd.start(object.CallArgs{}); err == nil {
		t.Fatal("failed command was started a second time")
	}
}
