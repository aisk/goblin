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
	obj, err := GoblinConstructor(CallArgs{
		Positional: append([]Object{&Function{Name: "fn", Fn: fn}}, args...),
	})
	if err != nil {
		t.Fatalf("Goblin(): %v", err)
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
	if got, _ := g.Done(CallArgs{}); got != False {
		t.Fatalf("done() before completion = %v, want false", got)
	}
	close(release)
	if _, err := g.Wait(CallArgs{}); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if got, _ := g.Done(CallArgs{}); got != True {
		t.Fatalf("done() after completion = %v, want true", got)
	}
}

func TestGoblinWaitTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	g := startGoblin(t, func(CallArgs) (Object, error) {
		<-release
		return Nil, nil
	})
	_, err := g.Wait(CallArgs{Keyword: map[string]Object{"timeout": Float(0.01)}})
	if !errors.Is(err, TimeoutError) {
		t.Fatalf("wait(timeout) error = %v, want TimeoutError", err)
	}
}

func TestGoblinWaitNilTimeoutMeansForever(t *testing.T) {
	g := startGoblin(t, func(CallArgs) (Object, error) {
		time.Sleep(10 * time.Millisecond)
		return Integer(3), nil
	})
	got, err := g.Wait(CallArgs{Keyword: map[string]Object{"timeout": Nil}})
	if err != nil || got != Integer(3) {
		t.Fatalf("wait(timeout=nil) = %v, %v", got, err)
	}
}

func TestGoblinWaitRejectsBadTimeout(t *testing.T) {
	g := startGoblin(t, func(CallArgs) (Object, error) { return Nil, nil })
	if _, err := g.Wait(CallArgs{Keyword: map[string]Object{"timeout": String("x")}}); err == nil {
		t.Fatal("wait(timeout=str) should fail")
	}
	if _, err := g.Wait(CallArgs{Keyword: map[string]Object{"timeout": Float(-1)}}); err == nil {
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

func TestGoblinConstructorRequiresFunction(t *testing.T) {
	if _, err := GoblinConstructor(CallArgs{}); err == nil {
		t.Fatal("Goblin() with no arguments should fail")
	}
	if _, err := GoblinConstructor(CallArgs{Positional: []Object{Integer(1)}}); err == nil {
		t.Fatal("Goblin(1) should fail")
	}
}
