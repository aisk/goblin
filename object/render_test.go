package object

import "testing"

// TestErrorElement: inside a collection an error shows as Kind("message"),
// Kind being the nearest kind down its cause chain, and a kind as its name;
// its direct display stays the bare message.
func TestErrorElement(t *testing.T) {
	configError := NewKindError(ValueError, "ConfigError")
	notFound := NewKindError(BaseError, "not found")
	cases := []struct {
		value Object
		want  string
	}{
		{NewKindError(ValueError, "bad input"), `ValueError("bad input")`},
		{NewKindError(ValueError, ""), `ValueError("")`},
		{NewKindError(KeyError, `a, "b"`), `KeyError("a, \"b\"")`},
		{NewKindError(BaseError, "x"), `Error("x")`},
		{ValueError, "ValueError"},
		{BaseError, "Error"},
		{configError, `ValueError("ConfigError")`},
		{NewKindError(configError, "missing"), `ValueError("missing")`},
		{notFound, `Error("not found")`},
		{NewKindError(notFound, "again"), `Error("again")`},
		{NewWrappedError("loading", NewKindError(ValueError, "bad")), `ValueError("loading: bad")`},
		{NewWrappedError("ctx", notFound), `Error("ctx: not found")`},
		{WithFrame(NewKindError(IndexError, "out of range"), Frame{Function: "f"}).(*Error), `IndexError("out of range")`},
		{WrapNativeError(IOError, "read x", errNotExistForTest{}), `IOError("read x: missing")`},
	}
	for _, c := range cases {
		l := &List{Elements: []Object{c.value}}
		if got := l.String(); got != "["+c.want+"]" {
			t.Errorf("[%s] renders as %s, want [%s]", inspect(c.value), got, c.want)
		}
		if got, err := l.ToString(); err != nil || got != "["+c.want+"]" {
			t.Errorf("ToString of [%s] = %s, %v", inspect(c.value), got, err)
		}
	}
	e := NewKindError(ValueError, "bad input")
	if s, _ := e.ToString(); s != "bad input" || e.String() != "bad input" {
		t.Errorf("direct display must stay the message, got %q", s)
	}
}

type errNotExistForTest struct{}

func (errNotExistForTest) Error() string { return "missing" }

// TestCallableArguments: every place that expects a function accepts what
// Call accepts, error kinds included, and names anything else as not
// callable.
func TestCallableArguments(t *testing.T) {
	if !Callable(ValueError) || !Callable(FunctionConstructorFn) || Callable(Integer(1)) {
		t.Fatal("Callable disagrees with Call")
	}
	got, err := FunctionConstructor(CallArgs{Positional: Args{ValueError}})
	if err != nil || got != ValueError {
		t.Fatalf("Function(ValueError) = %v, %v; want the kind unchanged", got, err)
	}
	if _, err := FunctionConstructor(CallArgs{Positional: Args{Integer(1)}}); err == nil || err.Error() != "Function() argument 'value' must be callable, got Int" {
		t.Fatalf("Function(1): %v", err)
	}
	l := &List{Elements: []Object{String("a")}}
	mapped, err := CallMethod(l, "map", CallArgs{Positional: Args{ValueError}})
	if err != nil {
		t.Fatal(err)
	}
	made := mapped.(*List).Elements[0].(*Error)
	if made.Value != "a" || made.Cause() != ValueError {
		t.Fatalf("map(ValueError) made %v with cause %v", made, made.Cause())
	}
	if _, err := CallMethod(l, "sort", CallArgs{Keyword: Kwargs{{Name: "key", Value: Integer(1)}}}); err == nil || err.Error() != "sort() argument 'key' must be callable or Nil, got Int" {
		t.Fatalf("sort(key=1): %v", err)
	}
	g, err := Spawn(CallArgs{Positional: Args{ValueError, String("x")}})
	if err != nil {
		t.Fatal(err)
	}
	if r, err := CallMethod(g, "wait", CallArgs{}); err != nil || r.(*Error).Value != "x" {
		t.Fatalf("spawn(ValueError, \"x\").wait() = %v, %v", r, err)
	}
	if _, err := Spawn(CallArgs{Positional: Args{Integer(1)}}); err == nil || err.Error() != "spawn() first argument must be callable, got Int" {
		t.Fatalf("spawn(1): %v", err)
	}
}
