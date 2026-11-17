// Package closeprobe provides close_probe, a test-only builtin whose passes
// report on stdout when they are closed. The early-exit tests register it in
// the interpreter's process and in a transpiled program alike, so both
// backends are checked against the same expected output.
package closeprobe

import (
	"fmt"
	"os"

	"github.com/aisk/goblin/extension"
	"github.com/aisk/goblin/object"
)

// Name is the builtin Register adds.
const Name = "close_probe"

// Register adds close_probe to the builtins.
//
// close_probe(label, n, fail_at=-1) is an iterable over 0 .. n-1 whose Next
// raises ValueError when it reaches fail_at; each pass prints
// "close <label>" when it is closed.
func Register() {
	mod := object.NewModule("closeprobe", nil, object.Funcs{Name: newProbe})
	extension.BuiltinsModule.Members[Name] = mod.Members[Name]
}

// Unregister removes close_probe again.
func Unregister() { delete(extension.BuiltinsModule.Members, Name) }

type probe struct {
	object.OpaqueBase
	label     string
	n, failAt int64
}

func newProbe(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser(Name, args)
	label, n, failAt := p.Str("label"), p.Int("n"), p.IntOr("fail_at", -1)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	return &probe{OpaqueBase: object.MakeOpaqueBase("Probe"), label: string(label), n: int64(n), failAt: int64(failAt)}, nil
}

func (p *probe) String() string            { return "<Probe " + p.label + ">" }
func (p *probe) ToString() (string, error) { return p.String(), nil }
func (p *probe) Attributes() []string      { return nil }
func (p *probe) GetAttr(name string) (object.Object, error) {
	return nil, object.NewAttributeError("Probe has no attribute '%s'", name)
}

func (p *probe) Iter() ([]object.Object, error) { return object.Collect(p) }

func (p *probe) Iterator() (object.Iterator, error) {
	i := int64(0)
	return object.IteratorWithClose(func() (object.Object, bool, error) {
		if i == p.failAt {
			return nil, false, object.NewValueError("probe %s failed at %d", p.label, i)
		}
		if i >= p.n {
			return nil, false, nil
		}
		i++
		return object.Integer(i - 1), true, nil
	}, func() {
		fmt.Fprintf(os.Stdout, "close %s\n", p.label)
	}), nil
}
