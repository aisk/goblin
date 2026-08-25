package html

import (
	"testing"

	"github.com/aisk/goblin/extension/internal/modtest"
	"github.com/aisk/goblin/object"
)

func TestHTML(t *testing.T) {
	escaped := modtest.CallModule(t, Execute, "escape", object.CallArgs{Positional: object.Args{object.String(`<Goblin & "Go">`)}})
	if escaped != object.String("&lt;Goblin &amp; &#34;Go&#34;&gt;") {
		t.Fatalf("escape() = %v", escaped)
	}
	if got := modtest.CallModule(t, Execute, "unescape", object.CallArgs{Positional: object.Args{escaped}}); got != object.String(`<Goblin & "Go">`) {
		t.Fatalf("unescape() = %v", got)
	}
}
