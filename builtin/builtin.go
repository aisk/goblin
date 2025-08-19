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
