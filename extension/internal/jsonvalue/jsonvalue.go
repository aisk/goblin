// Package jsonvalue converts between JSON text and Goblin values. It is the
// json module's codec, shared with http (request bodies and Response.json())
// so neither module imports the other.
package jsonvalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

// Decode returns the value of the one JSON document in data. Objects become
// Dicts in document order; a duplicate key keeps its last value at the
// position of its first occurrence. A document that is not valid JSON, empty
// input and anything after the document other than whitespace are a
// ParseError naming the place as "line L, column C", which it also carries as
// the data `line` and `column`. funcname is the function to blame.
func Decode(funcname string, data []byte) (object.Object, error) {
	// Validate the whole input first: the scanner behind Unmarshal reports
	// the offset of every syntax error, trailing data included, while the
	// token reader below only has to build values from a valid document.
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		var syntax *json.SyntaxError
		if !errors.As(err, &syntax) {
			return nil, object.WrapError(object.ParseError, funcname+"() invalid JSON", err)
		}
		offset := int(syntax.Offset)
		what := "unexpected end of input"
		if syntax.Error() != "unexpected end of JSON input" && offset > 0 && offset <= len(data) {
			// The offset is just past the offending character.
			r, size := utf8.DecodeLastRune(data[:offset])
			offset -= size
			what = "unexpected " + strconv.Quote(string(r))
		} else {
			offset = len(data)
		}
		return nil, parseError(funcname, data, offset, what)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return (&reader{funcname: funcname, data: data, dec: dec}).value()
}

func parseError(funcname string, data []byte, offset int, what string) error {
	line, column := Position(data, offset)
	return object.NewParseError("%s() invalid JSON at line %d, column %d: %s", funcname, line, column, what).
		With("line", object.Integer(line)).
		With("column", object.Integer(column))
}

// Position turns a byte offset in data into a line and a column, both counted
// from 1; the column counts characters, not bytes.
func Position(data []byte, offset int) (line, column int) {
	if offset > len(data) {
		offset = len(data)
	}
	before := data[:offset]
	line = 1 + bytes.Count(before, []byte{'\n'})
	start := bytes.LastIndexByte(before, '\n') + 1
	return line, 1 + utf8.RuneCount(before[start:])
}

type reader struct {
	funcname string
	data     []byte
	dec      *json.Decoder
}

func (r *reader) value() (object.Object, error) {
	tok, err := r.dec.Token()
	if err != nil {
		return nil, object.WrapError(object.ParseError, r.funcname+"() invalid JSON", err)
	}
	switch t := tok.(type) {
	case nil:
		return object.Nil, nil
	case bool:
		return object.Bool(t), nil
	case string:
		return object.String(t), nil
	case json.Number:
		return r.number(t)
	case json.Delim:
		if t == '[' {
			var elements []object.Object
			for r.dec.More() {
				v, err := r.value()
				if err != nil {
					return nil, err
				}
				elements = append(elements, v)
			}
			if _, err := r.dec.Token(); err != nil {
				return nil, object.WrapError(object.ParseError, r.funcname+"() invalid JSON", err)
			}
			if elements == nil {
				elements = []object.Object{}
			}
			return &object.List{Elements: elements}, nil
		}
		d := object.NewDict()
		for r.dec.More() {
			key, err := r.dec.Token()
			if err != nil {
				return nil, object.WrapError(object.ParseError, r.funcname+"() invalid JSON", err)
			}
			v, err := r.value()
			if err != nil {
				return nil, err
			}
			if err := d.Set(object.String(key.(string)), v); err != nil {
				return nil, err
			}
		}
		if _, err := r.dec.Token(); err != nil {
			return nil, object.WrapError(object.ParseError, r.funcname+"() invalid JSON", err)
		}
		return d, nil
	}
	return nil, object.NewParseError("%s() invalid JSON: unexpected token %v", r.funcname, tok)
}

// number is an Int when the text has no fraction or exponent and fits 64
// bits, a Float otherwise.
func (r *reader) number(n json.Number) (object.Object, error) {
	if i, err := n.Int64(); err == nil {
		return object.Integer(i), nil
	}
	f, err := n.Float64()
	if err != nil {
		// Only a magnitude beyond Float's range gets here; the token has
		// just been read, so it ends at the decoder's offset.
		offset := int(r.dec.InputOffset()) - len(n)
		return nil, parseError(r.funcname, r.data, offset, "number "+string(n)+" is out of range")
	}
	return object.Float(f), nil
}

// Encode returns the JSON text of v: compact unless pretty, in which case
// every element goes on its own line, indented by indent spaces per level,
// with ": " after keys. Dict
// entries keep the Dict's order. A value with a to_json() method is encoded as
// what that method returns; anything else without a JSON form is a TypeError.
func Encode(funcname string, v object.Object, indent int, pretty bool) (string, error) {
	e := &encoder{funcname: funcname, indent: indent, pretty: pretty}
	if err := e.value(v, 0); err != nil {
		return "", err
	}
	return e.buf.String(), nil
}

type encoder struct {
	funcname string
	indent   int
	pretty   bool
	buf      bytes.Buffer
}

func (e *encoder) value(obj object.Object, level int) error {
	// A value that contains itself would recurse forever; the stack check
	// turns that into a RecursionError.
	if err := object.CheckStack(); err != nil {
		return err
	}
	switch v := obj.(type) {
	case object.Unit:
		e.buf.WriteString("null")
		return nil
	case object.Bool:
		if v {
			e.buf.WriteString("true")
		} else {
			e.buf.WriteString("false")
		}
		return nil
	case object.Integer:
		e.buf.WriteString(strconv.FormatInt(int64(v), 10))
		return nil
	case object.Float:
		if math.IsInf(float64(v), 0) || math.IsNaN(float64(v)) {
			return object.NewValueError("%s() cannot encode %s (JSON has no NaN or infinity)", e.funcname, v.String())
		}
		// Float's own text is valid JSON for every finite value and keeps
		// the ".0" that makes 2.0 decode back as a Float.
		e.buf.WriteString(v.String())
		return nil
	case object.String:
		e.str(string(v))
		return nil
	case *object.List:
		return e.list(v.Elements, level)
	case *object.Dict:
		return e.dict(v, level)
	}
	method, err := obj.GetAttr("to_json")
	if err != nil {
		return object.NewTypeError("%s() cannot encode %s (give its type a to_json() method)", e.funcname, obj.TypeName())
	}
	encoded, err := object.Call(method, object.CallArgs{})
	if err != nil {
		return err
	}
	if encoded == obj {
		return object.NewValueError("%s() %s.to_json() returned the value itself", e.funcname, obj.TypeName())
	}
	return e.value(encoded, level)
}

func (e *encoder) str(s string) {
	// Marshal of a Go string cannot fail; it escapes what JSON requires and
	// replaces invalid UTF-8.
	b, _ := json.Marshal(s)
	e.buf.Write(b)
}

func (e *encoder) newline(level int) {
	if e.pretty {
		e.buf.WriteByte('\n')
		for i := 0; i < e.indent*level; i++ {
			e.buf.WriteByte(' ')
		}
	}
}

func (e *encoder) list(elements []object.Object, level int) error {
	if len(elements) == 0 {
		e.buf.WriteString("[]")
		return nil
	}
	e.buf.WriteByte('[')
	for i, el := range elements {
		if i > 0 {
			e.buf.WriteByte(',')
		}
		e.newline(level + 1)
		if err := e.value(el, level+1); err != nil {
			return err
		}
	}
	e.newline(level)
	e.buf.WriteByte(']')
	return nil
}

func (e *encoder) dict(d *object.Dict, level int) error {
	if d.Len() == 0 {
		e.buf.WriteString("{}")
		return nil
	}
	e.buf.WriteByte('{')
	for i, entry := range d.Entries() {
		key, ok := entry.Key.(object.String)
		if !ok {
			return object.NewTypeError("%s() Dict keys must be String, got %s", e.funcname, entry.Key.TypeName())
		}
		if i > 0 {
			e.buf.WriteByte(',')
		}
		e.newline(level + 1)
		e.str(string(key))
		if e.pretty {
			e.buf.WriteString(": ")
		} else {
			e.buf.WriteByte(':')
		}
		if err := e.value(entry.Value, level+1); err != nil {
			return err
		}
	}
	e.newline(level)
	e.buf.WriteByte('}')
	return nil
}
