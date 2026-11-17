package object

import (
	"errors"
	"testing"
)

func TestSlice(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2), Integer(3)}}
	tests := []struct {
		name      string
		obj       Object
		low, high Object
		want      string
	}{
		{"list both bounds", list, Integer(1), Integer(3), "[2, 3]"},
		{"list no low", list, nil, Integer(1), "[1]"},
		{"list no high", list, Integer(2), nil, "[3]"},
		{"list no bounds", list, nil, nil, "[1, 2, 3]"},
		{"list empty at the end", list, Integer(3), nil, "[]"},
		{"string counts characters", String("日本語abc"), Integer(1), Integer(4), "本語a"},
		{"string to the end", String("日本語"), Integer(2), nil, "語"},
		{"string empty", String(""), nil, nil, ""},
		{"bytes", Bytes("goblin"), Integer(2), Integer(4), `b"bl"`},
	}
	for _, tt := range tests {
		got, err := Slice(tt.obj, tt.low, tt.high)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if text, _ := got.ToString(); text != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, text, tt.want)
		}
	}
}

func TestSliceCopies(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2)}}
	got, err := Slice(list, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got.(*List).Elements[0] = Integer(9)
	if list.Elements[0] != Integer(1) {
		t.Errorf("slicing a list shares its storage")
	}

	raw := Bytes("ab")
	sliced, err := Slice(raw, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sliced.(Bytes)[0] = 'z'
	if raw[0] != 'a' {
		t.Errorf("slicing Bytes shares its storage")
	}
}

func TestSliceErrors(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2), Integer(3)}}
	tests := []struct {
		name      string
		obj       Object
		low, high Object
		kind      *Error
	}{
		{"high past the end", list, nil, Integer(4), IndexError},
		{"low before the start", list, Integer(-4), nil, IndexError},
		{"low above high", list, Integer(2), Integer(1), IndexError},
		{"string past the end", String("日本"), nil, Integer(3), IndexError},
		{"non-integer bound", list, String("a"), nil, TypeError},
		{"float bound", list, nil, Float(1), TypeError},
		{"not sliceable", Integer(1), nil, nil, TypeError},
		{"dict", NewDict(), nil, nil, TypeError},
	}
	for _, tt := range tests {
		_, err := Slice(tt.obj, tt.low, tt.high)
		if !errors.Is(err, tt.kind) {
			t.Errorf("%s: got %v, want a %s", tt.name, err, tt.kind.Error())
		}
	}
}

func TestUnpack(t *testing.T) {
	items, err := Unpack(String("ab"), 2)
	if err != nil || len(items) != 2 || items[1] != String("b") {
		t.Errorf("Unpack(\"ab\", 2) = %v, %v", items, err)
	}
	if _, err := Unpack(String("ab"), 3); !errors.Is(err, ValueError) {
		t.Errorf("too few elements: got %v, want a ValueError", err)
	}
	if _, err := Unpack(Integer(1), 2); !errors.Is(err, TypeError) {
		t.Errorf("not iterable: got %v, want a TypeError", err)
	}
}

func TestSliceNegativeBounds(t *testing.T) {
	list := &List{Elements: []Object{Integer(1), Integer(2), Integer(3)}}
	got, err := Slice(list, Integer(-2), nil)
	if err != nil || inspect(got) != "[2, 3]" {
		t.Errorf("list[-2:] = %v, %v", got, err)
	}
	got, err = Slice(String("héllo"), Integer(1), Integer(-1))
	if err != nil || got != String("éll") {
		t.Errorf(`"héllo"[1:-1] = %v, %v`, got, err)
	}
	_, err = Slice(list, Integer(-1), Integer(-2))
	if err == nil || err.Error() != "List slice bounds out of range: [-1:-2] with size 3" {
		t.Errorf("list[-1:-2] error = %v", err)
	}
}
