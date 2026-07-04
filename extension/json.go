package extension

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
)

func ExecuteJson() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"marshal":   &object.Function{Name: "marshal", Fn: jsonMarshal},
			"unmarshal": &object.Function{Name: "unmarshal", Fn: jsonUnmarshal},
		},
	}, nil
}

func jsonUnmarshal(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("unmarshal", args)
	s := ap.Str("s")
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(strings.NewReader(string(s)))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, object.WrapError(object.ParseError, "unmarshal() failed", err)
	}
	if dec.More() {
		return nil, object.NewValueError("unmarshal() unexpected trailing data after JSON value")
	}
	return JSONToGoblin(v)
}

// JSONToGoblin converts a value decoded by encoding/json (with UseNumber) into
// the corresponding goblin object. It is exported so other modules (e.g. http)
// can reuse it for their own JSON decoding.
func JSONToGoblin(v any) (object.Object, error) {
	switch x := v.(type) {
	case nil:
		return object.Unit{}, nil
	case bool:
		return object.Bool(x), nil
	case string:
		return object.String(x), nil
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return object.Integer(i), nil
		}
		f, err := x.Float64()
		if err != nil {
			return nil, object.WrapError(object.ParseError, "unmarshal() failed", err)
		}
		return object.Float(f), nil
	case []any:
		elements := make([]object.Object, 0, len(x))
		for _, item := range x {
			g, err := JSONToGoblin(item)
			if err != nil {
				return nil, err
			}
			elements = append(elements, g)
		}
		return &object.List{Elements: elements}, nil
	case map[string]any:
		d := object.NewDict()
		for k, val := range x {
			g, err := JSONToGoblin(val)
			if err != nil {
				return nil, err
			}
			d.Set(object.String(k), g)
		}
		return d, nil
	}
	return nil, object.NewTypeError("unmarshal() unsupported JSON value: %T", v)
}

func jsonMarshal(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("marshal", args)
	v := ap.Any("value")
	indent := int(int64(ap.IntOr("indent", 0)))
	if err := ap.Finish(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := goblinToJSON(v, &buf, indent, 0); err != nil {
		return nil, err
	}
	return object.String(buf.String()), nil
}

func goblinToJSON(obj object.Object, buf *bytes.Buffer, indent, level int) error {
	switch v := obj.(type) {
	case object.Unit:
		buf.WriteString("null")
		return nil
	case object.Bool:
		if bool(v) {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case object.Integer:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
		return nil
	case object.Float:
		b, err := json.Marshal(float64(v))
		if err != nil {
			return object.WrapError(object.ValueError, "marshal() failed", err)
		}
		buf.Write(b)
		return nil
	case object.String:
		b, err := json.Marshal(string(v))
		if err != nil {
			return object.WrapError(object.ValueError, "marshal() failed", err)
		}
		buf.Write(b)
		return nil
	case *object.List:
		return goblinListToJSON(v.Elements, buf, indent, level)
	case *object.Dict:
		return goblinDictToJSON(v, buf, indent, level)
	default:
		return object.NewTypeError("marshal() unsupported type: %T", obj)
	}
}

func goblinListToJSON(elements []object.Object, buf *bytes.Buffer, indent, level int) error {
	if len(elements) == 0 {
		buf.WriteString("[]")
		return nil
	}
	pretty := indent > 0
	buf.WriteByte('[')
	for i, e := range elements {
		if i > 0 {
			buf.WriteByte(',')
		}
		if pretty {
			buf.WriteByte('\n')
			writeSpaces(buf, indent*(level+1))
		}
		if err := goblinToJSON(e, buf, indent, level+1); err != nil {
			return err
		}
	}
	if pretty {
		buf.WriteByte('\n')
		writeSpaces(buf, indent*level)
	}
	buf.WriteByte(']')
	return nil
}

func goblinDictToJSON(d *object.Dict, buf *bytes.Buffer, indent, level int) error {
	if len(d.Entries) == 0 {
		buf.WriteString("{}")
		return nil
	}
	pretty := indent > 0
	buf.WriteByte('{')
	i := 0
	for _, entry := range d.Entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		i++
		if pretty {
			buf.WriteByte('\n')
			writeSpaces(buf, indent*(level+1))
		}
		kb, err := json.Marshal(entry.Key.String())
		if err != nil {
			return object.WrapError(object.ValueError, "marshal() failed", err)
		}
		buf.Write(kb)
		if pretty {
			buf.WriteString(": ")
		} else {
			buf.WriteByte(':')
		}
		if err := goblinToJSON(entry.Value, buf, indent, level+1); err != nil {
			return err
		}
	}
	if pretty {
		buf.WriteByte('\n')
		writeSpaces(buf, indent*level)
	}
	buf.WriteByte('}')
	return nil
}

func writeSpaces(buf *bytes.Buffer, n int) {
	for i := 0; i < n; i++ {
		buf.WriteByte(' ')
	}
}
