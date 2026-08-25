// Package digest holds the argument plumbing shared by the digest and
// checksum stdlib modules (x/crypto/* and x/hash/*).
package digest

import (
	"encoding/hex"

	"github.com/aisk/goblin/object"
)

// Data parses the single bytes-like data argument every digest function takes.
func Data(name string, args object.CallArgs) ([]byte, error) {
	p := object.NewArgParser(name, args)
	data := p.BytesLike("data")
	return data, p.Finish()
}

// Function builds a module function returning the raw digest as Bytes.
func Function(name string, digest func([]byte) []byte) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		data, err := Data(name, args)
		if err != nil {
			return nil, err
		}
		return object.NewBytes(digest(data)), nil
	}}
}

// HexFunction builds a module function returning the hex-encoded digest.
func HexFunction(name string, digest func([]byte) []byte) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		data, err := Data(name, args)
		if err != nil {
			return nil, err
		}
		return object.String(hex.EncodeToString(digest(data))), nil
	}}
}

// Module builds the two-member module shape shared by the fixed digests:
// sum returns Bytes, hex returns the hex string.
func Module(name string, sum func([]byte) []byte) *object.Module {
	return &object.Module{Name: name, Members: map[string]object.Object{
		"sum": Function("sum", sum),
		"hex": HexFunction("hex", sum),
	}}
}
