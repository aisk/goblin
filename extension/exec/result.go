package exec

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/aisk/goblin/object"
)

// Result is how a command ended: its exit status and, from capture, its
// output as text.
type Result struct {
	object.OpaqueBase
	code           int
	stdout, stderr object.Object
}

func itoa(n int) string { return strconv.Itoa(n) }

// text is captured output as a String; nil when the output was not
// captured. capture checks that its output is valid UTF-8 before it returns
// a Result; the output an ExitError or TimeoutError carries is diagnostic
// and has invalid UTF-8 replaced by U+FFFD, since raising a second error
// while building the first would hide it.
func text(buf *bytes.Buffer) object.Object {
	if buf == nil {
		return object.Nil
	}
	return object.String(strings.ToValidUTF8(buf.String(), "�"))
}

func newResult(code int, stdout, stderr *bytes.Buffer) *Result {
	return &Result{OpaqueBase: object.MakeOpaqueBase("Result"), code: code, stdout: text(stdout), stderr: text(stderr)}
}

func (r *Result) Constructor() *object.Function { return resultType.Function }
func (r *Result) String() string                { return "<Result code=" + itoa(r.code) + ">" }
func (r *Result) ToString() (string, error)     { return r.String(), nil }

func (r *Result) GetAttr(name string) (object.Object, error) {
	switch name {
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

func (r *Result) Attributes() []string { return []string{"code", "success", "stdout", "stderr"} }
