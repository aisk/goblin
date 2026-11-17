package object

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func startGoblin(t *testing.T, fn func(CallArgs) (Object, error), args ...Object) *Goblin {
	t.Helper()
	obj, err := Spawn(CallArgs{
		Positional: append([]Object{&Function{Name: "fn", Fn: fn}}, args...),
	})
	if err != nil {
		t.Fatalf("spawn(): %v", err)
	}
	return obj.(*Goblin)
}

func TestGoblinWaitReturnsResult(t *testing.T) {
	g := startGoblin(t, func(args CallArgs) (Object, error) {
		n := args.Positional[0].(Integer)
		return n * n, nil
	}, Integer(7))
	got, err := g.Wait(CallArgs{})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if got != Integer(49) {
		t.Fatalf("wait = %v, want 49", got)
	}
}

func TestGoblinWaitRaisesStoredError(t *testing.T) {
	raised := NewValueError("boom")
	g := startGoblin(t, func(CallArgs) (Object, error) {
		return nil, raised
	})
	for i := 0; i < 2; i++ {
		_, err := g.Wait(CallArgs{})
		if err != raised {
			t.Fatalf("wait #%d error = %v, want the raised error by identity", i+1, err)
		}
	}
}

func TestGoblinWaitConcurrentWaiters(t *testing.T) {
	release := make(chan struct{})
	g := startGoblin(t, func(CallArgs) (Object, error) {
		<-release
		return Integer(1), nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := g.Wait(CallArgs{}); err != nil || got != Integer(1) {
				t.Errorf("wait = %v, %v", got, err)
			}
		}()
	}
	close(release)
	wg.Wait()
}

func TestGoblinDone(t *testing.T) {
	release := make(chan struct{})
	g := startGoblin(t, func(CallArgs) (Object, error) {
		<-release
		return Nil, nil
	})
	if got, _ := g.GetAttr("done"); got != False {
		t.Fatalf("done before completion = %v, want false", got)
	}
	close(release)
	if _, err := g.Wait(CallArgs{}); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if got, _ := g.GetAttr("done"); got != True {
		t.Fatalf("done after completion = %v, want true", got)
	}
}

func TestGoblinWaitTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	g := startGoblin(t, func(CallArgs) (Object, error) {
		<-release
		return Nil, nil
	})
	_, err := g.Wait(CallArgs{Keyword: Kwargs{{Name: "timeout", Value: Float(0.01)}}})
	if !errors.Is(err, TimeoutError) {
		t.Fatalf("wait(timeout) error = %v, want TimeoutError", err)
	}
}

func TestGoblinWaitNilTimeoutMeansForever(t *testing.T) {
	g := startGoblin(t, func(CallArgs) (Object, error) {
		time.Sleep(10 * time.Millisecond)
		return Integer(3), nil
	})
	got, err := g.Wait(CallArgs{Keyword: Kwargs{{Name: "timeout", Value: Nil}}})
	if err != nil || got != Integer(3) {
		t.Fatalf("wait(timeout=nil) = %v, %v", got, err)
	}
}

func TestGoblinWaitRejectsBadTimeout(t *testing.T) {
	g := startGoblin(t, func(CallArgs) (Object, error) { return Nil, nil })
	if _, err := g.Wait(CallArgs{Keyword: Kwargs{{Name: "timeout", Value: String("x")}}}); err == nil {
		t.Fatal("wait(timeout=str) should fail")
	}
	if _, err := g.Wait(CallArgs{Keyword: Kwargs{{Name: "timeout", Value: Float(-1)}}}); err == nil {
		t.Fatal("wait(timeout=-1) should fail")
	}
}

func TestGoblinContainsPanic(t *testing.T) {
	g := startGoblin(t, func(CallArgs) (Object, error) {
		panic("kaboom")
	})
	_, err := g.Wait(CallArgs{})
	if !errors.Is(err, InternalError) {
		t.Fatalf("wait after panic = %v, want InternalError", err)
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("error %q should mention the panic value", err.Error())
	}
}

func TestSpawnRequiresFunction(t *testing.T) {
	if _, err := Spawn(CallArgs{}); err == nil {
		t.Fatal("spawn() with no arguments should fail")
	}
	if _, err := Spawn(CallArgs{Positional: []Object{Integer(1)}}); err == nil {
		t.Fatal("spawn(1) should fail")
	}
	// fn is positional-only: a keyword named fn goes to the function.
	if _, err := Spawn(CallArgs{Keyword: Kwargs{{Name: "fn", Value: Integer(1)}}}); err == nil {
		t.Fatal("spawn(fn=1) should fail")
	}
}

func TestSpawnForwardsKeywordArguments(t *testing.T) {
	fn := &Function{Name: "fn", Fn: func(args CallArgs) (Object, error) {
		p := NewArgParser("fn", args)
		a, b := p.Int("a"), p.Int("fn")
		if err := p.Finish(); err != nil {
			return nil, err
		}
		return a*10 + b, nil
	}}
	obj, err := Spawn(CallArgs{
		Positional: Args{fn, Integer(1)},
		Keyword:    Kwargs{{Name: "fn", Value: Integer(2)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := obj.(*Goblin).Wait(CallArgs{})
	if err != nil || got != Integer(12) {
		t.Fatalf("wait = %v, %v; want 12", got, err)
	}
}

func TestReportUnwaitedErrors(t *testing.T) {
	var discard strings.Builder
	ReportUnwaitedErrors(&discard)
	waited := startGoblin(t, func(CallArgs) (Object, error) { return nil, NewValueError("waited") })
	if _, err := waited.Wait(CallArgs{}); err == nil {
		t.Fatal("wait should raise")
	}
	first := startGoblin(t, func(CallArgs) (Object, error) { return nil, NewValueError("first") })
	<-first.done
	second := startGoblin(t, func(CallArgs) (Object, error) { panic("second") })
	<-second.done
	ok := startGoblin(t, func(CallArgs) (Object, error) { return Nil, nil })
	<-ok.done
	// A timed-out wait does not observe anything.
	release := make(chan struct{})
	defer close(release)
	running := startGoblin(t, func(CallArgs) (Object, error) { <-release; return nil, NewValueError("late") })
	running.Wait(CallArgs{Keyword: Kwargs{{Name: "timeout", Value: Integer(0)}}})

	var out strings.Builder
	ReportUnwaitedErrors(&out)
	want := "uncaught error in goblin: first\nuncaught error in goblin: panic in goblin: second\n"
	if out.String() != want {
		t.Errorf("report = %q, want %q", out.String(), want)
	}
	out.Reset()
	ReportUnwaitedErrors(&out)
	if out.String() != "" {
		t.Errorf("second report = %q, want nothing", out.String())
	}
}

func waitAny(goblins []Object, timeout Object) (Object, error) {
	args := CallArgs{Positional: Args{&List{Elements: goblins}}}
	if timeout != nil {
		args.Keyword = Kwargs{{Name: "timeout", Value: timeout}}
	}
	return WaitAny(args)
}

func TestWaitAny(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	blocked := startGoblin(t, func(CallArgs) (Object, error) { <-release; return Nil, nil })
	a := startGoblin(t, func(CallArgs) (Object, error) { return nil, NewValueError("a fails") })
	b := startGoblin(t, func(CallArgs) (Object, error) { return Integer(2), nil })
	<-a.done
	<-b.done
	// Both finished: the earliest in the list wins, and its error is neither
	// raised nor counted as observed.
	got, err := waitAny([]Object{blocked, b, a}, nil)
	if err != nil || got != b {
		t.Fatalf("wait_any = %v, %v; want b", got, err)
	}
	if got, err := waitAny([]Object{blocked, a, b}, Integer(0)); err != nil || got != a {
		t.Fatalf("wait_any(timeout=0) = %v, %v; want a", got, err)
	}
	if _, err := a.Wait(CallArgs{}); err == nil {
		t.Fatal("a should fail")
	}
	_, err = waitAny([]Object{blocked}, Float(0.01))
	if !errors.Is(err, TimeoutError) || err.Error() != "wait_any() timed out after 0.01 seconds" {
		t.Fatalf("wait_any timeout error = %v", err)
	}
	late := startGoblin(t, func(CallArgs) (Object, error) { time.Sleep(5 * time.Millisecond); return Nil, nil })
	if got, err := waitAny([]Object{blocked, late}, nil); err != nil || got != late {
		t.Fatalf("blocking wait_any = %v, %v; want late", got, err)
	}
	if _, err := waitAny([]Object{}, nil); !errors.Is(err, ValueError) {
		t.Fatalf("wait_any([]) error = %v, want ValueError", err)
	}
	if _, err := waitAny([]Object{b, Integer(1)}, nil); !errors.Is(err, TypeError) {
		t.Fatalf("wait_any([b, 1]) error = %v, want TypeError", err)
	}
}

func TestChanTimeouts(t *testing.T) {
	ch := NewChan(1)
	timeout := func(v Object) Kwargs { return Kwargs{{Name: "timeout", Value: v}} }
	// An operation that can complete does, even with timeout=0.
	if _, err := ch.Send(CallArgs{Positional: Args{Integer(1)}, Keyword: timeout(Integer(0))}); err != nil {
		t.Fatalf("send(timeout=0) on a free buffer: %v", err)
	}
	_, err := ch.Send(CallArgs{Positional: Args{Integer(2)}, Keyword: timeout(Float(0.01))})
	if !errors.Is(err, TimeoutError) || err.Error() != "send() timed out after 0.01 seconds" {
		t.Fatalf("send on a full channel = %v", err)
	}
	if v, err := ch.Receive(CallArgs{Keyword: timeout(Integer(0))}); err != nil || v != Integer(1) {
		t.Fatalf("receive(timeout=0) = %v, %v", v, err)
	}
	_, err = ch.Receive(CallArgs{Keyword: timeout(Integer(0))})
	if !errors.Is(err, TimeoutError) || err.Error() != "receive() timed out after 0 seconds" {
		t.Fatalf("receive on an empty channel = %v", err)
	}
	if _, err := ch.Receive(CallArgs{Keyword: timeout(Integer(-1))}); !errors.Is(err, ValueError) {
		t.Fatalf("receive(timeout=-1) = %v, want ValueError", err)
	}
}

func TestChanIteratesUntilClosed(t *testing.T) {
	ch := NewChan(3)
	for i := 1; i <= 3; i++ {
		ch.ch <- Integer(i)
	}
	close(ch.ch)
	items, err := Collect(ch)
	if err != nil || inspect(&List{Elements: items}) != "[1, 2, 3]" {
		t.Fatalf("Collect(chan) = %v, %v", items, err)
	}
}
