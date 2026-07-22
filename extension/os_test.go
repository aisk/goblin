package extension

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/aisk/goblin/object"
)

func osFunction(t *testing.T, name string) *object.Function {
	t.Helper()

	modObj, err := ExecuteOs()
	if err != nil {
		t.Fatalf("ExecuteOs() error = %v", err)
	}

	mod, ok := modObj.(*object.Module)
	if !ok {
		t.Fatalf("ExecuteOs() returned %T", modObj)
	}

	member, ok := mod.Members[name]
	if !ok {
		t.Fatalf("os module missing %q", name)
	}

	fn, ok := member.(*object.Function)
	if !ok {
		t.Fatalf("os module member %q is %T", name, member)
	}

	return fn
}

func argvFrom(modObj object.Object) (*object.Function, error) {
	mod, ok := modObj.(*object.Module)
	if !ok {
		return nil, fmt.Errorf("os module is %T", modObj)
	}
	fn, ok := mod.Members["argv"].(*object.Function)
	if !ok {
		return nil, fmt.Errorf("argv is %T", mod.Members["argv"])
	}
	return fn, nil
}

func argvStrings(fn *object.Function) ([]string, error) {
	got, err := fn.Call(object.CallArgs{})
	if err != nil {
		return nil, err
	}
	list, ok := got.(*object.List)
	if !ok {
		return nil, fmt.Errorf("argv() returned %T, want *object.List", got)
	}
	out := make([]string, len(list.Elements))
	for i, elem := range list.Elements {
		s, ok := elem.(object.String)
		if !ok {
			return nil, fmt.Errorf("argv()[%d] is %T, want object.String", i, elem)
		}
		out[i] = string(s)
	}
	return out, nil
}

func argvFromT(t *testing.T, modObj object.Object) *object.Function {
	t.Helper()
	fn, err := argvFrom(modObj)
	if err != nil {
		t.Fatal(err)
	}
	return fn
}

func callArgv(t *testing.T, fn *object.Function) []string {
	t.Helper()
	got, err := argvStrings(fn)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestOsArgv(t *testing.T) {
	fn := osFunction(t, "argv")

	got := callArgv(t, fn)
	if len(got) != len(os.Args) {
		t.Fatalf("argv() size = %d, want %d", len(got), len(os.Args))
	}
	for i, s := range got {
		if s != os.Args[i] {
			t.Fatalf("argv()[%d] = %q, want %q", i, s, os.Args[i])
		}
	}

	if _, err := fn.Call(object.CallArgs{Positional: object.Args{object.Integer(1)}}); err == nil {
		t.Fatal("argv(1) expected error")
	}
	if _, err := fn.Call(object.CallArgs{Keyword: object.Kwargs{"x": object.Integer(1)}}); err == nil {
		t.Fatal("argv(x=1) expected error")
	}
}

func TestExecuteOsWithFrozenArgs(t *testing.T) {
	want := []string{"script.goblin", "foo", "bar"}
	modObj, err := ExecuteOsWithFrozenArgs(want)
	if err != nil {
		t.Fatalf("ExecuteOsWithFrozenArgs() error = %v", err)
	}
	fn := argvFromT(t, modObj)

	got := callArgv(t, fn)
	if len(got) != len(want) {
		t.Fatalf("argv() size = %d, want %d", len(got), len(want))
	}
	for i, s := range got {
		if s != want[i] {
			t.Fatalf("argv()[%d] = %q, want %q", i, s, want[i])
		}
	}

	// Live ExecuteOs() is independent of WithFrozenArgs snapshots.
	live := callArgv(t, osFunction(t, "argv"))
	if len(live) != len(os.Args) {
		t.Fatalf("ExecuteOs argv size = %d, want %d", len(live), len(os.Args))
	}
	for i, s := range live {
		if s != os.Args[i] {
			t.Fatalf("ExecuteOs argv[%d] = %q, want %q", i, s, os.Args[i])
		}
	}

	// Snapshot is copied: mutating the input slice must not change argv().
	want[1] = "mutated"
	got = callArgv(t, fn)
	if got[1] != "foo" {
		t.Fatalf("argv()[1] = %q after input mutation, want %q", got[1], "foo")
	}
}

func TestExecuteOsWithFrozenArgsConcurrent(t *testing.T) {
	const n = 32
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			label := strconv.Itoa(i)
			modObj, err := ExecuteOsWithFrozenArgs([]string{label, "x"})
			if err != nil {
				errCh <- err
				return
			}
			fn, err := argvFrom(modObj)
			if err != nil {
				errCh <- err
				return
			}
			got, err := argvStrings(fn)
			if err != nil {
				errCh <- err
				return
			}
			if len(got) != 2 || got[0] != label || got[1] != "x" {
				errCh <- fmt.Errorf("unexpected argv %q for label %q", got, label)
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}
