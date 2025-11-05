package builtin

import (
	"fmt"

	"github.com/aisk/goblin/object"
)

func Print(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	for _, arg := range args {
		fmt.Print(arg.String(), " ")
	}
	fmt.Print("\n")
	return nil, nil
}

func Range(args []object.Object, kwargs map[string]object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("range() takes exactly 2 arguments, got %d", len(args))
	}

	start, ok := args[0].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("range() first argument must be an integer, got %T", args[0])
	}

	end, ok := args[1].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("range() second argument must be an integer, got %T", args[1])
	}

	if int64(start) >= int64(end) {
		return &object.List{Elements: []object.Object{}}, nil
	}

	elements := make([]object.Object, int64(end)-int64(start))
	for i := int64(start); i < int64(end); i++ {
		elements[i-int64(start)] = object.Integer(i)
	}

	return &object.List{Elements: elements}, nil
}
