package exec

import (
	"errors"
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
	value, err := command(object.CallArgs{
		Positional: []object.Object{object.String(os.Args[0]), &object.List{Elements: argObjects}},
		Keyword: map[string]object.Object{
			"env": &object.Dict{Entries: map[string]object.DictEntry{
				"GO_WANT_HELPER_PROCESS": {Key: object.String("GO_WANT_HELPER_PROCESS"), Value: object.String("1")},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return value.(*Cmd)
}

func TestOutputAndCombinedOutput(t *testing.T) {
	output, err := helperCommand(t, "out").output(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(output.(object.Bytes)); got != "out" {
		t.Fatalf("output = %q", got)
	}
	combined, err := helperCommand(t, "out", "err").combinedOutput(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(combined.(object.Bytes)); got != "outerr" && got != "errout" {
		t.Fatalf("combined_output = %q", got)
	}
}

func TestRunAndWaitReturnExitErrors(t *testing.T) {
	if _, err := helperCommand(t, "", "", "fail").run(object.CallArgs{}); err == nil || !errors.Is(err, object.IOError) {
		t.Fatalf("run error = %v, want IOError", err)
	}
	cmd := helperCommand(t)
	if _, err := cmd.start(object.CallArgs{}); err != nil {
		t.Fatal(err)
	}
	if _, err := cmd.wait(object.CallArgs{}); err != nil {
		t.Fatal(err)
	}
}

func TestLookPath(t *testing.T) {
	value, err := lookPath(object.CallArgs{Positional: object.Args{object.String("go")}})
	if err != nil {
		t.Fatal(err)
	}
	if value.(object.String) == "" {
		t.Fatal("look_path returned an empty path")
	}
	if _, err := lookPath(object.CallArgs{Positional: object.Args{object.String("definitely-not-a-goblin-command")}}); err == nil {
		t.Fatal("look_path accepted a missing executable")
	}
}
