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
		Name: "json",
		Members: map[string]object.Object{
			"marshal":   &object.Function{Name: "marshal", Fn: jsonMarshal},
			"unmarshal": &object.Function{Name: "unmarshal", Fn: jsonUnmarshal},
		},
	}, nil
}

func jsonUnmarshal(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("unmarshal", args)
	value := ap.Any("data")
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	var data string
	switch v := value.(type) {
	case object.String:
		data = string(v)
	case object.Bytes:
		data = string(v)
	default:
		return nil, object.NewTypeError("unmarshal() argument 'data' must be str or Bytes, got %s", value.TypeName())
	}

	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, object.WrapError(object.ParseError, "unmarshal() invalid JSON", err)
	}
	if dec.More() {
		return nil, object.NewValueError("unmarshal() unexpected trailing data after JSON value")
	}
	return JSONToGoblin(v, "unmarshal")
}

// JSONToGoblin converts a value decoded by encoding/json (with UseNumber) into
// the corresponding goblin object. It is exported so other modules (e.g. http)
// can reuse it for their own JSON decoding; funcname is the user-visible
// function to blame in error messages.
func JSONToGoblin(v any, funcname string) (object.Object, error) {
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
			return nil, object.WrapError(object.ParseError, funcname+"() invalid JSON number", err)
		}
		return object.Float(f), nil
	case []any:
		elements := make([]object.Object, 0, len(x))
		for _, item := range x {
			g, err := JSONToGoblin(item, funcname)
			if err != nil {
				return nil, err
			}
			elements = append(elements, g)
		}
		return &object.List{Elements: elements}, nil
	case map[string]any:
		d := object.NewDict()
		for k, val := range x {
			g, err := JSONToGoblin(val, funcname)
			if err != nil {
				return nil, err
			}
			if err := d.Set(object.String(k), g); err != nil {
				return nil, err
			}
		}
		return d, nil
	}
	return nil, object.NewTypeError("%s() unsupported JSON value: %T", funcname, v)
}

func jsonMarshal(args object.CallArgs) (object.Object, error) {
	ap := object.NewArgParser("marshal", args)
	v := ap.Any("value")
	indent := int(ap.IntOr("indent", 0))
	if err := ap.Finish(); err != nil {
		return nil, err
	}
	if indent < 0 {
		return nil, object.NewValueError("marshal() argument 'indent' must be non-negative, got %d", indent)
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
			return object.WrapError(object.ValueError, "marshal() unsupported float value", err)
		}
		buf.Write(b)
		return nil
	case object.String:
		b, err := json.Marshal(string(v))
		if err != nil {
			return object.WrapError(object.ValueError, "marshal() invalid string", err)
		}
		buf.Write(b)
		return nil
	case *object.List:
		return goblinListToJSON(v.Elements, buf, indent, level)
	case *object.Dict:
		return goblinDictToJSON(v, buf, indent, level)
	default:
		return object.NewTypeError("marshal() unsupported type: %s", obj.TypeName())
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
	if d.Len() == 0 {
		buf.WriteString("{}")
		return nil
	}
	pretty := indent > 0
	buf.WriteByte('{')
	i := 0
	for _, entry := range d.Entries() {
		if i > 0 {
			buf.WriteByte(',')
		}
		i++
		if pretty {
			buf.WriteByte('\n')
			writeSpaces(buf, indent*(level+1))
		}
		key, ok := entry.Key.(object.String)
		if !ok {
			return object.NewTypeError("marshal() dict keys must be str, got %s", entry.Key.TypeName())
		}
		kb, err := json.Marshal(string(key))
		if err != nil {
			return object.WrapError(object.ValueError, "marshal() invalid dict key", err)
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
