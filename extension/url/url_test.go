package url

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestParseAndResolve(t *testing.T) {
	value, err := parse(object.CallArgs{Positional: object.Args{object.String("https://example.com:8443/a?q=1#f")}})
	if err != nil {
		t.Fatal(err)
	}
	u := value.(*URL)
	if u.value.Hostname() != "example.com" || u.value.Port() != "8443" {
		t.Fatalf("parsed URL = %#v", u.value)
	}
	ref, _ := parse(object.CallArgs{Positional: object.Args{object.String("../b")}})
	resolved, err := u.resolveReference(object.CallArgs{Positional: object.Args{ref}})
	if err != nil || resolved.(*URL).value.Path != "/b" {
		t.Fatalf("resolved = %v, %v", resolved, err)
	}
}
