package extension

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aisk/goblin/object"
)

var RandomModule = &object.Module{
	Members: map[string]object.Object{
		"int":   &object.Function{Name: "int", Fn: RandInt},
		"intn":  &object.Function{Name: "intn", Fn: RandIntn},
		"float": &object.Function{Name: "float", Fn: RandFloat},
	},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandInt(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("int() requires no arguments")
	}
	return object.Integer(rand.Int63()), nil
}

func RandIntn(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("intn() requires exactly 1 argument")
	}
	n, ok := args[0].(object.Integer)
	if !ok {
		return nil, fmt.Errorf("intn() argument must be an integer, got %T", args[0])
	}
	if int64(n) <= 0 {
		return nil, fmt.Errorf("intn() argument must be positive, got %d", n)
	}
	return object.Integer(rand.Int63n(int64(n))), nil
}

func RandFloat(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("float() requires no arguments")
	}
	return object.Float(rand.Float64()), nil
}
