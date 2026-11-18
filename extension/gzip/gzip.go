// Package gzip implements the gzip module: whole values in gzip format, and
// .gz files of any size as Files and lazy lines.
package gzip

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"os"

	"github.com/aisk/goblin/extension/internal/filestream"
	"github.com/aisk/goblin/extension/internal/pathvalue"
	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("gzip", nil, object.Funcs{
		"compress":   compress,
		"decompress": decompress,
		"open":       open,
		"read_lines": readLines,
	}), nil
}

// levelArg checks an optional compression level: 1 to 9, or nil for Go's
// default.
func levelArg(fn string, level object.Integer, hasLevel bool) (int, error) {
	if !hasLevel {
		return gzip.DefaultCompression, nil
	}
	if level < gzip.BestSpeed || level > gzip.BestCompression {
		return 0, object.RangeArgumentError(fn, "level", 1, 9, int64(level))
	}
	return int(level), nil
}

func compress(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("compress", args)
	data := p.Data("data")
	level, hasLevel := p.NullableInt("level")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	goLevel, err := levelArg("compress", level, hasLevel)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, goLevel)
	if err != nil {
		return nil, object.WrapError(object.ValueError, "compress() invalid level", err)
	}
	if _, err := io.Copy(w, data); err != nil {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return nil, err
		}
		return nil, object.WrapNativeError(object.IOError, "compress() cannot read data", err)
	}
	if err := w.Close(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "compress() cannot write", err)
	}
	return object.NewBytes(out.Bytes()), nil
}

// decompress takes Bytes or a readable; gzip data is binary, so a String is
// rejected rather than read as its UTF-8 bytes.
func decompress(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("decompress", args)
	value := p.Any("data")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var input io.Reader
	if _, isString := value.(object.String); !isString {
		input, _ = object.DataReader("decompress", "data", value)
	}
	if input == nil {
		return nil, object.NewTypeError("decompress() argument 'data' must be Bytes or have a read(size) method, got %s", value.TypeName())
	}
	r, err := gzip.NewReader(input)
	if err != nil {
		return nil, decompressError(err, true)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, decompressError(err, false)
	}
	return object.NewBytes(out), nil
}

// decompressError passes on an error the readable raised and reports any
// other failure as a ParseError: input that does not start like gzip is "not
// gzip data", anything wrong after that is "corrupt gzip data".
func decompressError(err error, atStart bool) error {
	var goblinErr *object.Error
	if errors.As(err, &goblinErr) {
		return err
	}
	// Input too short to hold a header is not gzip either, as openGzip
	// decides for a file.
	if atStart && (errors.Is(err, gzip.ErrHeader) || err == io.EOF || err == io.ErrUnexpectedEOF) {
		return object.NewParseError("decompress() not gzip data")
	}
	return object.NewParseError("decompress() corrupt gzip data")
}

// openGzip opens the .gz file at path for reading, with its header already
// read: a file that does not start like gzip is a ParseError "not gzip
// data" carrying path. The reader maps errors in the compressed stream to
// corruptError.
func openGzip(fn, path string) (io.Reader, io.Closer, error) {
	file, err := openFile(path)
	if err != nil {
		return nil, nil, filestream.Fail(fn, "cannot open", path, err)
	}
	zr, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		if errors.Is(err, gzip.ErrHeader) || err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, nil, object.NewParseError("%s() %s is not gzip data", fn, filestream.Quote(path)).
				With("path", pathvalue.NewPath(path))
		}
		return nil, nil, filestream.Fail(fn, "cannot read", path, err)
	}
	return &corruptMapper{r: zr, path: path}, file, nil
}

// openFile opens a .gz file for reading; tests replace it to see the file
// closed.
var openFile = func(path string) (io.ReadCloser, error) { return os.Open(path) }

// corruptMapper reports damage found in the compressed stream as a
// corruptError, which names the Goblin call that reached it.
type corruptMapper struct {
	r    io.Reader
	path string
}

func (m *corruptMapper) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	if err != nil && err != io.EOF && isCorrupt(err) {
		return n, corruptError{path: m.path}
	}
	return n, err
}

func isCorrupt(err error) bool {
	var corrupt flate.CorruptInputError
	return errors.Is(err, gzip.ErrChecksum) || errors.Is(err, gzip.ErrHeader) ||
		errors.As(err, &corrupt) || errors.Is(err, io.ErrUnexpectedEOF)
}

// corruptError is damaged gzip data found while reading a .gz file. It is a
// filestream.MethodError: the ParseError it becomes names the call.
type corruptError struct{ path string }

func (e corruptError) Error() string { return "corrupt gzip data in " + filestream.Quote(e.path) }

func (e corruptError) ForMethod(method string) error {
	var err *object.Error
	if method == "" {
		err = object.NewParseError("corrupt gzip data in %s", filestream.Quote(e.path))
	} else {
		err = object.NewParseError("%s() corrupt gzip data in %s", method, filestream.Quote(e.path))
	}
	return err.With("path", pathvalue.NewPath(e.path))
}

// readLines is fs.read_lines over the decompressed text of a .gz file.
func readLines(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("read_lines", args)
	path := p.Path("path")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return filestream.NewLines("read_lines", path, func() (io.Reader, io.Closer, error) {
		return openGzip("read_lines", path)
	}), nil
}

// open is fs.open for a .gz file: a File that reads the decompressed
// content, or with write=true compresses what is written into the file.
func open(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("open", args)
	path := p.Path("path")
	writing := bool(p.BoolOr("write", false))
	level, hasLevel := p.NullableInt("level")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	if hasLevel && !writing {
		return nil, object.NewValueError("open() argument 'level' needs write=true")
	}
	goLevel, err := levelArg("open", level, hasLevel)
	if err != nil {
		return nil, err
	}
	if !writing {
		r, c, err := openGzip("open", path)
		if err != nil {
			return nil, err
		}
		return filestream.OpenReader(path, r, c), nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, filestream.Fail("open", "cannot open", path, err)
	}
	zw, err := gzip.NewWriterLevel(file, goLevel)
	if err != nil {
		file.Close()
		return nil, object.WrapError(object.ValueError, "open() invalid level", err)
	}
	return filestream.OpenWriter(path, zw, closeBoth{zw, file}), nil
}

// closeBoth finishes the gzip data, then closes the file under it; the
// first error wins.
type closeBoth struct {
	zw   *gzip.Writer
	file *os.File
}

func (c closeBoth) Close() error {
	err := c.zw.Close()
	if closeErr := c.file.Close(); err == nil {
		err = closeErr
	}
	return err
}
