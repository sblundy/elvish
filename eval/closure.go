package eval

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/elves/elvish/eval/vals"
	"github.com/elves/elvish/eval/vars"
	"github.com/elves/elvish/parse"
	"github.com/elves/elvish/util"
	"github.com/xiaq/persistent/hash"
)

// ErrArityMismatch is thrown by a closure when the number of arguments the user
// supplies does not match with what is required.
var ErrArityMismatch = errors.New("arity mismatch")

// Closure is a closure defined in Elvish script. Each closure has its unique
// identity.
type Closure struct {
	ArgNames []string
	// The name for the rest argument. If empty, the function has fixed arity.
	RestArg     string
	OptNames    []string
	OptDefaults []interface{}
	Op          effectOp
	Captured    Ns
	SrcMeta     *Source
	DefBegint   int
	DefEnd      int
}

var _ Callable = &Closure{}

// Kind returns "fn".
func (*Closure) Kind() string {
	return "fn"
}

// Equal compares by address.
func (c *Closure) Equal(rhs interface{}) bool {
	return c == rhs
}

// Hash returns the hash of the address of the closure.
func (c *Closure) Hash() uint32 {
	return hash.Pointer(unsafe.Pointer(c))
}

// Repr returns an opaque representation "<closure 0x23333333>".
func (c *Closure) Repr(int) string {
	return fmt.Sprintf("<closure %p>", c)
}

// Index supports the introspection of the closure. Supported keys are
// "arg-names", "rest-arg", "opt-names", "opt-defaults", "body", "def" and
// "src".
func (c *Closure) Index(k interface{}) (interface{}, bool) {
	switch k {
	case "arg-names":
		return listOfStrings(c.ArgNames), true
	case "rest-arg":
		return c.RestArg, true
	case "opt-names":
		return listOfStrings(c.OptNames), true
	case "opt-defaults":
		return vals.MakeList(c.OptDefaults...), true
	case "body":
		return c.SrcMeta.code[c.Op.begin:c.Op.end], true
	case "def":
		return c.SrcMeta.code[c.DefBegint:c.DefEnd], true
	case "src":
		return c.SrcMeta, true
	}
	return nil, false
}

func listOfStrings(ss []string) vals.List {
	list := vals.EmptyList
	for _, s := range ss {
		list = list.Cons(s)
	}
	return list
}

// IterateKeys calls f with all the valid keys that can be used for Index.
func (c *Closure) IterateKeys(f func(interface{}) bool) {
	util.Feed(f, "arg-names", "rest-arg",
		"opt-names", "opt-defaults", "body", "def", "src")
}

// Call calls a closure.
func (c *Closure) Call(fm *Frame, args []interface{}, opts map[string]interface{}) error {
	if c.RestArg != "" {
		if len(c.ArgNames) > len(args) {
			return fmt.Errorf("need %d or more arguments, got %d", len(c.ArgNames), len(args))
		}
	} else {
		if len(c.ArgNames) != len(args) {
			return fmt.Errorf("need %d arguments, got %d", len(c.ArgNames), len(args))
		}
	}

	// This evalCtx is dedicated to the current form, so we modify it in place.
	// BUG(xiaq): When evaluating closures, async access to global variables
	// and ports can be problematic.

	// Make upvalue namespace and capture variables.
	// TODO(xiaq): Is it safe to simply assign ec.up = c.Captured?
	fm.up = make(Ns)
	for name, variable := range c.Captured {
		fm.up[name] = variable
	}

	// Populate local scope with arguments, possibly a rest argument, and
	// options.
	fm.local = make(Ns)
	for i, name := range c.ArgNames {
		fm.local[name] = vars.FromInit(args[i])
	}
	if c.RestArg != "" {
		fm.local[c.RestArg] = vars.FromInit(vals.MakeList(args[len(c.ArgNames):]...))
	}
	optUsed := make(map[string]struct{})
	for i, name := range c.OptNames {
		v, ok := opts[name]
		if ok {
			optUsed[name] = struct{}{}
		} else {
			v = c.OptDefaults[i]
		}
		fm.local[name] = vars.FromInit(v)
	}
	for name := range opts {
		_, used := optUsed[name]
		if !used {
			return fmt.Errorf("unknown option %s", parse.Quote(name))
		}
	}

	fm.traceback = fm.addTraceback()

	fm.srcMeta = c.SrcMeta
	return c.Op.exec(fm)
}
