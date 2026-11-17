package csv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

var rowsType = object.NewOpaqueConstructor("Rows", "csv.read_rows(readable)")

// Rows is what csv.read_rows returns: each pass reads rows from the
// readable it was given, one at a time, by the rules decode follows. It
// never closes the readable: whoever opened it does.
type Rows struct {
	object.OpaqueBase
	source io.Reader
	sep    rune
}

// read_rows(readable, sep=",") reads CSV from anything with a read(size)
// method, lazily.
func readRows(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("read_rows", args)
	readable := p.Any("readable")
	sepValue := p.StrOr("sep", ",")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	sep, err := separator("read_rows", sepValue)
	if err != nil {
		return nil, err
	}
	source, err := object.NewDuckReader("read_rows", "readable", readable)
	if err != nil {
		return nil, err
	}
	// The DuckReader's Close is never called: the readable is not ours.
	return &Rows{OpaqueBase: object.MakeOpaqueBase("Rows"), source: source, sep: sep}, nil
}

func (r *Rows) Constructor() *object.Function { return rowsType.Function }
func (r *Rows) String() string                { return "<Rows>" }
func (r *Rows) ToString() (string, error)     { return r.String(), nil }
func (r *Rows) Attributes() []string          { return nil }
func (r *Rows) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Rows has no attribute '%s'", name)
}

func (r *Rows) Iter() ([]object.Object, error) { return object.Collect(r) }

// Iterator starts a pass where the readable now is. A byte order mark at
// that point is dropped, as decode drops one at the start of its data.
func (r *Rows) Iterator() (object.Iterator, error) {
	buffered := bufio.NewReader(r.source)
	if head, _ := buffered.Peek(len(byteOrderMark)); bytes.Equal(head, byteOrderMark) {
		buffered.Discard(len(byteOrderMark))
	}
	keeper := &lineKeeper{r: buffered, first: 1}
	reader := newReader(keeper, r.sep)
	done := false
	return object.IteratorFunc(func() (object.Object, bool, error) {
		if done {
			return nil, false, nil
		}
		record, err := reader.Read()
		if err == io.EOF {
			done = true
			return nil, false, nil
		}
		if err != nil {
			done = true
			var pe *csv.ParseError
			if !errors.As(err, &pe) {
				// The readable's own failure, raised as it is.
				return nil, false, err
			}
			atEnd := keeper.offset(pe.Line, pe.Column) >= len(bytes.TrimRight(keeper.buf, "\r\n"))
			return nil, false, errorAt("read_rows", pe.Line, keeper.column(pe.Line, pe.Column), problem(pe, atEnd))
		}
		for i, field := range record {
			if !utf8.ValidString(field) {
				done = true
				line, column := reader.FieldPos(i)
				return nil, false, keeper.invalidUTF8(line, column)
			}
		}
		// Nothing before this record is needed again.
		line, _ := reader.FieldPos(0)
		keeper.drop(line)
		return stringList(record), true, nil
	}), nil
}

// lineKeeper passes the readable through to encoding/csv and keeps what it
// delivered from the start of line first on, so an error can be placed by
// character column. Lines before the current record are dropped, which
// bounds it by the size of a record plus the read-ahead.
type lineKeeper struct {
	r     io.Reader
	buf   []byte
	first int
}

func (k *lineKeeper) Read(p []byte) (int, error) {
	n, err := k.r.Read(p)
	k.buf = append(k.buf, p[:n]...)
	return n, err
}

// drop forgets the lines before line.
func (k *lineKeeper) drop(line int) {
	for k.first < line {
		next := bytes.IndexByte(k.buf, '\n')
		if next < 0 {
			return
		}
		k.buf = k.buf[next+1:]
		k.first++
	}
}

// lineStart is the offset in buf where line begins.
func (k *lineKeeper) lineStart(line int) int {
	start := 0
	for i := k.first; i < line; i++ {
		next := bytes.IndexByte(k.buf[start:], '\n')
		if next < 0 {
			return len(k.buf)
		}
		start += next + 1
	}
	return start
}

// offset is the offset in buf of a line and 1-based byte column.
func (k *lineKeeper) offset(line, byteColumn int) int {
	return k.lineStart(line) + byteColumn - 1
}

// column converts a 1-based byte column in line to a count of characters.
func (k *lineKeeper) column(line, byteColumn int) int {
	start := k.lineStart(line)
	end := min(start+byteColumn-1, len(k.buf))
	return 1 + utf8.RuneCount(k.buf[start:end])
}

// invalidUTF8 is the ParseError for the first invalid byte at or after the
// start of a field, given as a line and a byte column.
func (k *lineKeeper) invalidUTF8(line, byteColumn int) error {
	from := k.offset(line, byteColumn)
	at := from + max(invalidUTF8(k.buf[from:]), 0)
	start := k.lineStart(line)
	for {
		next := bytes.IndexByte(k.buf[start:at], '\n')
		if next < 0 {
			break
		}
		start += next + 1
		line++
	}
	return errorAt("read_rows", line, 1+utf8.RuneCount(k.buf[start:at]), "invalid UTF-8")
}
