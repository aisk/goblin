package path

import (
	"testing"

	"github.com/aisk/goblin/object"
)

func TestPathTypeName(t *testing.T) {
	if got := NewPath("/tmp").TypeName(); got != "Path" {
		t.Errorf("TypeName() = %q, want %q", got, "Path")
	}
}

func TestPathAttributesReturnsResolvableNames(t *testing.T) {
	p := NewPath(".")
	for _, name := range p.Attributes() {
		if _, err := p.GetAttr(name); err != nil {
			t.Errorf("Path.Attributes() contains %q, but GetAttr failed: %v", name, err)
		}
	}
}

func TestPathString(t *testing.T) {
	if got, ok := PathString(object.String("/tmp/x")); !ok || got != "/tmp/x" {
		t.Errorf("PathString(String) = %q, %v; want %q, true", got, ok, "/tmp/x")
	}
	if got, ok := PathString(NewPath("/tmp/x")); !ok || got != "/tmp/x" {
		t.Errorf("PathString(*Path) = %q, %v; want %q, true", got, ok, "/tmp/x")
	}
	if _, ok := PathString(object.Integer(1)); ok {
		t.Error("PathString(Integer) accepted a non-path value")
	}
}
