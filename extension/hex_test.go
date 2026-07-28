package extension

import (
	"errors"
	"testing"

	"github.com/aisk/goblin/object"
)

func TestHexRoundTrip(t *testing.T) {
	encoded, err := hexEncode(object.CallArgs{Positional: object.Args{object.NewBytes([]byte{0, 0xff})}})
	if err != nil || encoded != object.String("00ff") {
		t.Fatalf("encode = %v, %v", encoded, err)
	}
	decoded, err := hexDecode(object.CallArgs{Positional: object.Args{encoded}})
	if err != nil || !objectEquals(decoded, object.NewBytes([]byte{0, 0xff})) {
		t.Fatalf("decode = %v, %v", decoded, err)
	}
	if _, err := hexDecode(object.CallArgs{Positional: object.Args{object.String("xyz")}}); err == nil || !errors.Is(err, object.ParseError) {
		t.Fatalf("invalid decode error = %v", err)
	}
}
