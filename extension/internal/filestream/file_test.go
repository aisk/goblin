package filestream

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("disk on fire") }

type goblinFailure struct{}

func (goblinFailure) Error() string { return "broken" }
func (goblinFailure) ForMethod(method string) error {
	return object.NewParseError("%s() broken", method)
}

func callMethod(t *testing.T, f *File, name string) error {
	t.Helper()
	m, err := f.GetAttr(name)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.(*object.Function).Call(object.CallArgs{})
	return err
}

func TestIOErrorsCarryThePath(t *testing.T) {
	f := OpenReader("data.bin", failingReader{}, nil)
	err := callMethod(t, f, "read")
	var goblinErr *object.Error
	if !errors.As(err, &goblinErr) || !errors.Is(err, object.IOError) {
		t.Fatalf("read error = %v, want IOError", err)
	}
	if p, ok := goblinErr.Field("path"); !ok || p.(*pathvalue.Path).FilePath() != "data.bin" {
		t.Fatalf("path = %v", p)
	}
	if !strings.Contains(err.Error(), `read() failed to read "data.bin"`) {
		t.Fatalf("message = %q", err)
	}

	// A stream without a path has no data.
	err = callMethod(t, NewReader("pipe", failingReader{}, nil), "read")
	errors.As(err, &goblinErr)
	if _, ok := goblinErr.Field("path"); ok {
		t.Fatalf("a pipe's error carries a path: %v", err)
	}
}

func TestMethodErrorNamesTheCall(t *testing.T) {
	f := OpenReader("x.gz", io.MultiReader(strings.NewReader("a\n"), readerErr{goblinFailure{}}), nil)
	if err := callMethod(t, f, "read"); err == nil || err.Error() != "read() broken" {
		t.Fatalf("read error = %v", err)
	}
	lines := NewLines("read_lines", "x.gz", func() (io.Reader, io.Closer, error) {
		return io.MultiReader(strings.NewReader("a\n"), readerErr{goblinFailure{}}), io.NopCloser(nil), nil
	})
	_, err := object.Collect(lines)
	if err == nil || err.Error() != "read_lines() broken" || !errors.Is(err, object.ParseError) {
		t.Fatalf("read_lines error = %v", err)
	}
}

type readerErr struct{ err error }

func (r readerErr) Read([]byte) (int, error) { return 0, r.err }
