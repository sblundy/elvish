package newedit

import (
	"reflect"
	"testing"

	"github.com/elves/elvish/cli/clitypes"
	"github.com/elves/elvish/cli/listing"
	"github.com/elves/elvish/edit/ui"
	"github.com/elves/elvish/eval"
)

func TestInitLastCmd_Start(t *testing.T) {
	ed := &fakeApp{}
	ev := eval.NewEvaler()
	lsMode := listing.Mode{}
	lsBinding := emptyBindingMap

	ns := initLastcmd(ed, ev, testStore, &lsMode, &lsBinding)

	// Call <edit:listing>:start.
	fm := eval.NewTopFrame(ev, eval.NewInternalSource("[test]"), nil)
	fm.Call(getFn(ns, "start"), nil, eval.NoOpts)

	// Verify that the current mode supports listing.
	lister, ok := ed.state.Mode().(clitypes.Lister)
	if !ok {
		t.Errorf("Mode is not Lister after <edit:lastcmd>:start")
	}
	// Verify the listing.
	buf := ui.Render(lister.List(10), 20)
	wantBuf := ui.NewBufferBuilder(20).
		WriteString("    echo hello world", "7").Newline().
		WriteUnstyled("  0 echo").Newline().
		WriteUnstyled("  1 hello").Newline().
		WriteUnstyled("  2 world").Buffer()
	if !reflect.DeepEqual(buf, wantBuf) {
		t.Errorf("Rendered listing is %v, want %v", buf, wantBuf)
	}
}
