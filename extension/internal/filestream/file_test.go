package filestream

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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

// TestReadSizeFollowsTheData checks that read(size) reserves memory for the
// data that arrives, not for the size asked: a size near the Int limit
// returns what the stream holds instead of panicking.
func TestReadSizeFollowsTheData(t *testing.T) {
	read := func(f *File, size int64) object.Bytes {
		t.Helper()
		m, err := f.GetAttr("read")
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.(*object.Function).Call(object.CallArgs{Positional: object.Args{object.Integer(size)}})
		if err != nil {
			t.Fatal(err)
		}
		return got.(object.Bytes)
	}
	for _, size := range []int64{1 << 62, 1<<63 - 1, 1 << 40} {
		f := NewReader("pipe", strings.NewReader("small"), nil)
		if got := read(f, size); string(got) != "small" {
			t.Fatalf("read(%d) = %q", size, got)
		}
		if got := read(f, size); len(got) != 0 {
			t.Fatalf("read(%d) at the end = %q", size, got)
		}
	}

	// A file on disk still gives the whole size in one read.
	path := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(path, make([]byte, 200000), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f := Open(path, file, false)
	defer file.Close()
	for _, want := range []int{150000, 50000, 0} {
		if got := read(f, 150000); len(got) != want {
			t.Fatalf("read(150000) gave %d bytes, want %d", len(got), want)
		}
	}
}
