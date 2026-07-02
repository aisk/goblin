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
	bound, err := object.BindArguments("unmarshal", []string{"s"}, "", "", args)
	if err != nil {
		return nil, err
	}
	s, ok := bound["s"].(object.String)
	if !ok {
		return nil, object.NewTypeError("unmarshal() argument must be a string, got %T", bound["s"])
	}

	dec := json.NewDecoder(strings.NewReader(string(s)))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
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
			return nil, err
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
	if len(args.Positional) < 1 || len(args.Positional) > 2 {
		return nil, object.NewTypeError("marshal() takes 1 or 2 arguments, got %d", len(args.Positional))
	}

	indent := 0
	if len(args.Positional) == 2 {
		iv, ok := args.Positional[1].(object.Integer)
		if !ok {
			return nil, object.NewTypeError("marshal() indent must be an integer, got %T", args.Positional[1])
		}
		indent = int(int64(iv))
	}
	for k, v := range args.Keyword {
		if k != "indent" {
			return nil, object.NewValueError("marshal() got an unexpected keyword argument '%s'", k)
		}
		iv, ok := v.(object.Integer)
		if !ok {
			return nil, object.NewTypeError("marshal() indent must be an integer, got %T", v)
		}
		indent = int(int64(iv))
	}

	var buf bytes.Buffer
	if err := goblinToJSON(args.Positional[0], &buf, indent, 0); err != nil {
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
			return err
		}
		buf.Write(b)
		return nil
	case object.String:
		b, err := json.Marshal(string(v))
		if err != nil {
			return err
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
			return err
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
