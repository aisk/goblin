package exec

import (
	"fmt"
	"github.com/aisk/goblin/object"
)

type Result struct {
	objectBase
	code   int
	stdout object.Object
	stderr object.Object
}

func (r *Result) String() string              { return fmt.Sprintf("<exec.Result code=%d>", r.code) }
func (r *Result) ToString() (string, error)   { return r.String(), nil }
func (r *Result) Bool() bool                  { return r.code == 0 }
func (r *Result) ToBool() (bool, error)       { return r.Bool(), nil }
func (r *Result) Not() (object.Object, error) { return object.Bool(!r.Bool()), nil }
func (r *Result) GetAttr(name string) (object.Object, error) {
	switch name {
	case "attributes":
		return object.AttributesFunction(r), nil
	case "code":
		return object.Integer(r.code), nil
	case "success":
		return object.Bool(r.code == 0), nil
	case "stdout":
		return r.stdout, nil
	case "stderr":
		return r.stderr, nil
	}
	return nil, object.NewAttributeError("Result has no attribute '%s'", name)
}
func (r *Result) Attributes() []string {
	return []string{"attributes", "code", "success", "stdout", "stderr"}
}

var _ object.Object = (*Cmd)(nil)
var _ object.Object = (*Result)(nil)
var _ object.Object = (*streamPolicy)(nil)
