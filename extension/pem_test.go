package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestPEMRoundTrip(t *testing.T) {
	module, _ := ExecutePEM()
	constructor := module.(*object.Module).Members["Block"].(*object.Function)
	blockObj, err := constructor.Call(object.CallArgs{Positional: object.Args{object.String("MESSAGE"), object.String("Goblin")}})
	if err != nil {
		t.Fatal(err)
	}
	encodedFn, err := blockObj.GetAttr("encode")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodedFn.(*object.Function).Call(object.CallArgs{})
	if err != nil {
		t.Fatal(err)
	}
	decoded := callModuleFunction(t, ExecutePEM, "decode", object.CallArgs{Positional: object.Args{encoded}})
	items := decoded.(*object.List).Elements
	equal, err := blockObj.Equals(items[0])
	if err != nil || !equal || len(items[1].(object.Bytes)) != 0 {
		t.Fatalf("decode() = %v, equal=%v, err=%v", decoded, equal, err)
	}
}
