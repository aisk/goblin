// Package archive holds the plumbing shared by the x/archive modules:
// the ordered entry list, argument parsing, and the buffer-or-dest sink.
package archive

import (
	"bytes"
	"io"

	"github.com/aisk/goblin/object"
)

// Entry preserves the caller's dict order so that produced archives are
// deterministic and read_all round-trips entries in archive order.
type Entry struct {
	Name string
	Data []byte
}

// Data parses the single bytes-like data argument of the read_all functions.
func Data(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	data := p.BytesLike("data")
	return data, p.Finish()
}

// Files converts the files dict into ordered entries.
func Files(name string, value object.Object) ([]Entry, error) {
	dict, ok := value.(*object.Dict)
	if !ok {
		return nil, object.NewTypeError("%s() argument 'files' must be a dict, got %s", name, value.TypeName())
	}
	entries := make([]Entry, 0, dict.Len())
	for _, entry := range dict.Entries() {
		filename, ok := entry.Key.(object.String)
		if !ok {
			return nil, object.NewTypeError("%s() file names must be strings, got %s", name, entry.Key.TypeName())
		}
		switch content := entry.Value.(type) {
		case object.Bytes:
			entries = append(entries, Entry{string(filename), []byte(content)})
		case object.String:
			entries = append(entries, Entry{string(filename), []byte(content)})
		default:
			return nil, object.NewTypeError("%s() file %q must contain Bytes or str, got %s", name, filename, entry.Value.TypeName())
		}
	}
	return entries, nil
}

// Dict converts read entries back into the result dict.
func Dict(entries []Entry) (*object.Dict, error) {
	result := object.NewDict()
	for _, entry := range entries {
		if err := result.Set(object.String(entry.Name), object.NewBytes(entry.Data)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Dest resolves the optional dest keyword shared by the write_all functions:
// nil keeps buffering into output, anything else must be a duck writer that
// receives the archive bytes instead.
func Dest(destObj object.Object, output *bytes.Buffer) (io.Writer, error) {
	if _, ok := destObj.(object.Unit); ok {
		return output, nil
	}
	return object.NewDuckWriter("write_all", "dest", destObj)
}
