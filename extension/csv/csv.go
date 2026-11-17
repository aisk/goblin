// Package csv converts between CSV text and rows: a row is a List of Strings
// (or, by column name, a Dict) and a table is a List of rows. Whole documents
// only.
package csv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

func Execute() (object.Object, error) {
	return object.NewModule("csv", nil, object.Funcs{
		"decode":       decode,
		"decode_dicts": decodeDicts,
		"encode":       encode,
	}), nil
}

// byteOrderMark is dropped from the start of the input.
var byteOrderMark = []byte("\xef\xbb\xbf")

// separator checks the sep argument: one character that encoding/csv can use
// as a delimiter.
func separator(fn string, sep object.String) (rune, error) {
	text := string(sep)
	r, size := utf8.DecodeRuneInString(text)
	if text == "" || size != len(text) || r == utf8.RuneError || r == '"' || r == '\r' || r == '\n' {
		return 0, object.NewValueError("%s() argument 'sep' must be one character other than a quote or a line break, got %s", fn, strconv.Quote(text))
	}
	return r, nil
}

// table is one parsed document: its records and, for each field, the line
// and byte column it starts at, for error positions.
type table struct {
	fn      string
	data    []byte
	records [][]string
	fields  [][][2]int
}

// read parses the data argument of fn: every record, with the positions of
// its fields.
func read(fn string, args object.CallArgs) (*table, error) {
	p := object.NewArgParser(fn, args)
	data := p.DataBytes("data")
	sepValue := p.StrOr("sep", ",")
	if err := p.Finish(); err != nil {
		return nil, err
	}
	sep, err := separator(fn, sepValue)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, byteOrderMark)
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sep
	reader.FieldsPerRecord = -1
	t := &table{fn: fn, data: data}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return t, nil
		}
		if err != nil {
			return nil, t.parseError(err)
		}
		positions := make([][2]int, len(record))
		for i := range record {
			line, column := reader.FieldPos(i)
			positions[i] = [2]int{line, column}
		}
		t.records = append(t.records, record)
		t.fields = append(t.fields, positions)
	}
}

// parseError turns an encoding/csv error into a ParseError at the place it
// names, worded without Go's own message, which repeats the position.
func (t *table) parseError(err error) error {
	var pe *csv.ParseError
	if !errors.As(err, &pe) {
		return object.WrapError(object.ParseError, t.fn+"() invalid CSV", err)
	}
	what := "stray quote"
	if errors.Is(pe.Err, csv.ErrQuote) && t.offset(pe.Line, pe.Column) >= len(bytes.TrimRight(t.data, "\r\n")) {
		what = "quoted field never ends"
	} else if !errors.Is(pe.Err, csv.ErrQuote) && !errors.Is(pe.Err, csv.ErrBareQuote) {
		what = pe.Err.Error()
	}
	return t.errorAt(pe.Line, pe.Column, what)
}

// errorAt is the ParseError for a place given as encoding/csv gives it: a
// line and a 1-based byte column in that line.
func (t *table) errorAt(line, byteColumn int, what string) error {
	column := t.column(line, byteColumn)
	return object.NewParseError("%s() invalid CSV at line %d, column %d: %s", t.fn, line, column, what).
		With("line", object.Integer(line)).
		With("column", object.Integer(column))
}

// lineStart is the byte offset where line (from 1) begins.
func (t *table) lineStart(line int) int {
	start := 0
	for i := 1; i < line; i++ {
		next := bytes.IndexByte(t.data[start:], '\n')
		if next < 0 {
			return len(t.data)
		}
		start += next + 1
	}
	return start
}

// offset is the byte offset of a line and byte column.
func (t *table) offset(line, byteColumn int) int {
	return t.lineStart(line) + byteColumn - 1
}

// column converts a 1-based byte column in line to a 1-based count of
// characters.
func (t *table) column(line, byteColumn int) int {
	start := t.lineStart(line)
	end := min(start+byteColumn-1, len(t.data))
	return 1 + utf8.RuneCount(t.data[start:end])
}

func stringList(fields []string) *object.List {
	elements := make([]object.Object, len(fields))
	for i, field := range fields {
		elements[i] = object.String(field)
	}
	return &object.List{Elements: elements}
}

// decode(data, sep=",") returns every record as a List of Strings, each as
// long as the record is.
func decode(args object.CallArgs) (object.Object, error) {
	t, err := read("decode", args)
	if err != nil {
		return nil, err
	}
	rows := make([]object.Object, len(t.records))
	for i, record := range t.records {
		rows[i] = stringList(record)
	}
	return &object.List{Elements: rows}, nil
}

// decode_dicts(data, sep=",") reads the first record as the header and
// returns every later record as a Dict from column name to field, in header
// order. Every record must have as many fields as the header.
func decodeDicts(args object.CallArgs) (object.Object, error) {
	t, err := read("decode_dicts", args)
	if err != nil {
		return nil, err
	}
	if len(t.records) == 0 {
		return &object.List{Elements: []object.Object{}}, nil
	}
	header := t.records[0]
	seen := make(map[string]bool, len(header))
	for i, name := range header {
		if seen[name] {
			pos := t.fields[0][i]
			return nil, t.errorAt(pos[0], pos[1], "duplicate column "+strconv.Quote(name))
		}
		seen[name] = true
	}
	rows := make([]object.Object, 0, len(t.records)-1)
	for r, record := range t.records[1:] {
		if len(record) != len(header) {
			line := t.fields[r+1][0][0]
			return nil, object.NewParseError("decode_dicts() invalid CSV at line %d, column 1: expected %d fields, got %d", line, len(header), len(record)).
				With("line", object.Integer(line)).
				With("column", object.Integer(1))
		}
		d := object.NewDict()
		for i, name := range header {
			if err := d.Set(object.String(name), object.String(record[i])); err != nil {
				return nil, err
			}
		}
		rows = append(rows, d)
	}
	return &object.List{Elements: rows}, nil
}

// encode(rows, sep=",", columns=nil) returns CSV text, one "\n"-terminated
// line per row. Rows are Lists, written as they are, or Dicts, written under
// a header line of columns: the columns argument, else the first Dict's keys.
func encode(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("encode", args)
	rowsValue := p.Any("rows")
	sepValue := p.StrOr("sep", ",")
	columnsValue := p.AnyOr("columns", object.Nil)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	sep, err := separator("encode", sepValue)
	if err != nil {
		return nil, err
	}
	var columns []string
	haveColumns := false
	if columnsValue != object.Nil {
		list, ok := columnsValue.(*object.List)
		if !ok {
			return nil, object.NewTypeError("encode() argument 'columns' must be List or Nil, got %s", columnsValue.TypeName())
		}
		for _, c := range list.Elements {
			name, ok := c.(object.String)
			if !ok {
				return nil, object.NewTypeError("encode() argument 'columns' must hold Strings, got %s", c.TypeName())
			}
			columns = append(columns, string(name))
		}
		haveColumns = true
	}
	rows, err := object.CollectArg("encode", "rows", rowsValue)
	if err != nil {
		return nil, err
	}

	var out strings.Builder
	writer := csv.NewWriter(&out)
	writer.Comma = sep
	if !haveColumns && len(rows) > 0 {
		if first, ok := rows[0].(*object.Dict); ok {
			for _, entry := range first.Entries() {
				name, ok := entry.Key.(object.String)
				if !ok {
					return nil, object.NewTypeError("encode() row 1 has a column name that is %s, not String", entry.Key.TypeName())
				}
				columns = append(columns, string(name))
			}
			haveColumns = true
		}
	}
	if haveColumns {
		writer.Write(columns)
	}
	index := make(map[string]int, len(columns))
	for i, name := range columns {
		index[name] = i
	}
	for i, rowValue := range rows {
		n := i + 1
		var record []string
		switch row := rowValue.(type) {
		case *object.List:
			record = make([]string, len(row.Elements))
			for j, field := range row.Elements {
				if record[j], err = fieldText(n, field); err != nil {
					return nil, err
				}
			}
		case *object.Dict:
			if !haveColumns {
				return nil, object.NewValueError("encode() row %d is a Dict but the rows before it are Lists; give the columns with columns=", n)
			}
			record = make([]string, len(columns))
			for _, entry := range row.Entries() {
				name, isString := entry.Key.(object.String)
				j, known := index[string(name)]
				if !isString || !known {
					return nil, object.NewValueError("encode() row %d has unknown column %s", n, quote(entry.Key))
				}
				if record[j], err = fieldText(n, entry.Value); err != nil {
					return nil, err
				}
			}
		default:
			return nil, object.NewTypeError("encode() row %d must be List or Dict, got %s", n, rowValue.TypeName())
		}
		writer.Write(record)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "encode() cannot write CSV", err)
	}
	return object.String(out.String()), nil
}

// quote renders a Dict key for a message: a String in quotes, anything else
// as print shows it.
func quote(key object.Object) string {
	if s, ok := key.(object.String); ok {
		return strconv.Quote(string(s))
	}
	if s, ok := key.(interface{ String() string }); ok {
		return s.String()
	}
	return key.TypeName()
}

// fieldText is the text of one field of row n: a String as it is, nil as an
// empty field, a List or Dict an error, and anything else its String() form;
// a user value needs a Show impl for that.
func fieldText(n int, value object.Object) (string, error) {
	switch v := value.(type) {
	case object.String:
		return string(v), nil
	case object.Unit:
		return "", nil
	case *object.List, *object.Dict:
		return "", object.NewTypeError("encode() row %d cannot hold a %s as a field", n, value.TypeName())
	case object.UserValue:
		if !v.UserType().Implements(object.ShowProtocol) {
			return "", object.NewTypeError("encode() row %d cannot hold a %s as a field (add 'impl Show' to type %s)", n, value.TypeName(), value.TypeName())
		}
	}
	return value.ToString()
}
