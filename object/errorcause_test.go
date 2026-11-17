package object

import (
	"errors"
	"io/fs"
	"testing"
)

func TestErrorCauseAndOrigin(t *testing.T) {
	base := NewError("base")
	framed := WithFrame(WithFrame(NewWrappedError("ctx", base), Frame{Function: "f"}), Frame{Function: "g"}).(*Error)
	if got := framed.Cause(); got != base {
		t.Fatalf("cause through frames = %v, want base", got)
	}
	caught := WithFrame(WithFrame(base, Frame{Function: "f"}), Frame{Function: "g"}).(*Error)
	if eq, _ := caught.Equals(base); !eq {
		t.Fatal("a caught error should equal the raised one")
	}
	if h1, _ := caught.Hash(); h1 != identityHash(base) {
		t.Fatal("a caught error should hash like the raised one")
	}
	if caught.Cause() != Nil {
		t.Fatalf("cause of a framed root = %v, want nil", caught.Cause())
	}

	native := WrapError(IOError, "read() failed", fs.ErrNotExist)
	if !errors.Is(native, IOError) || !errors.Is(native, fs.ErrNotExist) {
		t.Fatal("the two-layer wrap must still match its kind and native error")
	}
	inner, ok := native.Cause().(*Error)
	if !ok || inner.Value != fs.ErrNotExist.Error() || !errors.Is(inner, IOError) {
		t.Fatalf("cause of a wrapped native error = %v", native.Cause())
	}
	if inner.Cause() != IOError {
		t.Fatalf("the native layer's cause = %v, want its kind", inner.Cause())
	}
	if got := WithFrame(fs.ErrNotExist, Frame{}).(*Error).Cause(); got != Nil {
		t.Fatalf("a framed native error's cause = %v, want nil", got)
	}
	plain := &Error{Value: "x", Wrapped: fs.ErrNotExist}
	if got, ok := plain.Cause().(*Error); !ok || got.Value != fs.ErrNotExist.Error() {
		t.Fatalf("a wrapped native error's cause = %v", plain.Cause())
	}
	if IndexError.Cause() != LookupError || BaseError.Cause() != Nil {
		t.Fatal("kinds chain to their parents")
	}
}
