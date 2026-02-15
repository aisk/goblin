package extension

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aisk/goblin/object"
)

func ExecuteRandom() (object.Object, error) {
	return &object.Module{
		Members: map[string]object.Object{
			"int":   &object.Function{Name: "int", Fn: randInt},
			"intn":  &object.Function{Name: "intn", Fn: randIntn},
			"float": &object.Function{Name: "float", Fn: randFloat},
		},
	}, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randInt(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("int() requires no arguments")
	}
	return object.Integer(rand.Int63()), nil
}

func randIntn(args object.Args, kwargs object.KwArgs) (object.Object, error) {
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

func randFloat(args object.Args, kwargs object.KwArgs) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("float() requires no arguments")
	}
	return object.Float(rand.Float64()), nil
}
