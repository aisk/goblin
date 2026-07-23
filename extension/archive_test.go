package extension

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestArchiveRoundTrips(t *testing.T) {
	files := object.NewDict()
	_ = files.Set(object.String("a.txt"), object.String("alpha"))
	_ = files.Set(object.String("dir/b.bin"), object.NewBytes([]byte{0, 1, 2}))
	for _, pair := range []struct {
		write, read func(object.CallArgs) (object.Object, error)
	}{{tarWriteAll, tarReadAll}, {zipWriteAll, zipReadAll}} {
		archive, err := pair.write(object.CallArgs{Positional: object.Args{files}})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := pair.read(object.CallArgs{Positional: object.Args{archive}})
		if err != nil {
			t.Fatal(err)
		}
		result := decoded.(*object.Dict)
		alpha, _, err := result.Get(object.String("a.txt"))
		if err != nil || string(alpha.(object.Bytes)) != "alpha" {
			t.Fatalf("text entry = %v, %v", alpha, err)
		}
		binary, _, err := result.Get(object.String("dir/b.bin"))
		if err != nil || !binary.Equals(object.NewBytes([]byte{0, 1, 2})) {
			t.Fatalf("binary entry = %v, %v", binary, err)
		}
	}
}
