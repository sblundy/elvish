package clicore

import (
	"fmt"
	"os"

	"github.com/elves/elvish/edit/tty"
	"github.com/elves/elvish/edit/ui"
	"github.com/elves/elvish/sys"
)

// TTY is the type the terminal dependency of the editor needs to satisfy.
type TTY interface {
	// Configures the terminal at the beginning of Editor.ReadCode. It returns a
	// restore function to be called at the end of Editor.ReadCode and any
	// error. Errors are always considered fatal and will make ReadCode abort;
	// non-fatal errors should be handled by Setup itself (e.g. by showing a
	// warning message) instead of being returned.
	Setup() (restore func(), err error)

	// Starts the delivery of terminal events and returns a channel on which
	// events are made available.
	StartInput() <-chan tty.Event
	// Sets the "raw input" mode of the terminal. The raw input mode is
	// applicable when terminal events are delivered as escape sequences; the
	// raw input mode will cause those escape sequences to be interpreted as
	// individual key events. If the concept is not applicable, this method is a
	// no-op.
	SetRawInput(raw bool)
	// Causes input delivery to be stopped. When this function returns, the
	// channel previously returned by StartInput should no longer deliver
	// events.
	StopInput()

	// Returns the height and width of the terminal.
	Size() (h, w int)
	// Outputs a newline.
	Newline()
	// Returns the current buffer. The initial value of the current buffer is
	// nil.
	Buffer() *ui.Buffer
	// Resets the current buffer to nil without actuating any redraw.
	ResetBuffer()
	// Updates the current buffer and draw it to the terminal.
	UpdateBuffer(bufNotes, bufMain *ui.Buffer, full bool) error
}

type aTTY struct {
	in, out *os.File
	r       tty.Reader
	w       tty.Writer
}

// NewTTY returns a new TTY from input and output terminal files.
func NewTTY(in, out *os.File) TTY {
	return &aTTY{in, out, nil, tty.NewWriter(out)}
}

func (t *aTTY) Setup() (func(), error) {
	restore, err := tty.Setup(t.in, t.out)
	return func() {
		err := restore()
		if err != nil {
			fmt.Println(t.out, "failed to restore terminal properties:", err)
		}
	}, err
}

func (t *aTTY) Size() (h, w int) {
	return sys.GetWinsize(t.out)
}

func (t *aTTY) StartInput() <-chan tty.Event {
	t.r = tty.NewReader(t.in)
	t.r.Start()
	return t.r.EventChan()
}

func (t *aTTY) SetRawInput(raw bool) {
	t.r.SetRaw(raw)
}

func (t *aTTY) StopInput() {
	t.r.Stop()
	t.r.Close()
	t.r = nil
}

func (t *aTTY) Newline() {
	t.w.Newline()
}

func (t *aTTY) Buffer() *ui.Buffer {
	return t.w.CurrentBuffer()
}

func (t *aTTY) ResetBuffer() {
	t.w.ResetCurrentBuffer()
}

func (t *aTTY) UpdateBuffer(bufNotes, bufMain *ui.Buffer, full bool) error {
	return t.w.CommitBuffer(bufNotes, bufMain, full)
}
