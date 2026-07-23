package extension

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestHexRoundTrip(t *testing.T) {
	encoded, err := hexEncodeToString(object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0, 0xff})}})
	if err != nil || encoded != object.String("00ff") {
		t.Fatalf("encode_to_string = %v, %v", encoded, err)
	}
	decoded, err := hexDecodeString(object.CallArgs{Positional: object.Args{encoded}})
	if err != nil || !decoded.Equals(object.NewBytes([]byte{0, 0xff})) {
		t.Fatalf("decode_string = %v, %v", decoded, err)
	}
	if _, err := hexDecodeString(object.CallArgs{Positional: object.Args{object.String("xyz")}}); err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("invalid decode error = %v", err)
	}
}
