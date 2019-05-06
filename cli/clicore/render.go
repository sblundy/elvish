package clicore

import (
	"github.com/elves/elvish/cli/clitypes"
	"github.com/elves/elvish/edit/ui"
	"github.com/elves/elvish/styled"
	"github.com/elves/elvish/util"
)

type renderSetup struct {
	height int
	width  int

	prompt  styled.Text
	rprompt styled.Text

	code   styled.Text
	dot    int
	errors []error

	notes []string

	mode clitypes.Mode
}

// Renders the editor state.
func render(r *renderSetup) (notes, main *ui.Buffer) {
	var bufNotes *ui.Buffer
	if len(r.notes) > 0 {
		bufNotes = ui.Render(&linesRenderer{r.notes}, r.width)
	}

	bbCode := ui.NewBufferBuilder(r.width)
	(&codeContentRenderer{r.code, r.dot, r.prompt, r.rprompt}).Render(bbCode)
	if len(r.errors) > 0 {
		bufCodeErrors := ui.Render(&codeErrorsRenderer{r.errors}, r.width)
		bbCode.Extend(bufCodeErrors, false)
	}
	bufCode := bbCode.Buffer()

	bbMain := ui.NewBufferBuilder(r.width)
	(&mainRenderer{r.height, bufCode, r.mode}).Render(bbMain)

	return bufNotes, bbMain.Buffer()
}

// Renderer of the entire editor. The code area and the status area needs to be
// pre-rendered, while the modeline and the listing area are rendered by
// mainRenderer by calling the methods in the Mode.
type mainRenderer struct {
	maxHeight int
	bufCode   *ui.Buffer
	mode      clitypes.Mode
}

func (r *mainRenderer) Render(buf *ui.BufferBuilder) {
	bufCode := r.bufCode
	mode := getMode(r.mode)
	bufMode := ui.Render(mode.ModeLine(), buf.Width)

	// Determine which parts to render and the available height for the listing.
	hListing := 0
	switch {
	case r.maxHeight >= ui.BuffersHeight(bufCode, bufMode):
		// Both the code area and the modeline fits. Use the remaining height
		// for the listing.
		hListing = r.maxHeight - ui.BuffersHeight(bufCode, bufMode)
	case r.maxHeight >= ui.BuffersHeight(bufMode)+1:
		// The modeline fits and there is at least one line for the code area.
		// As a special case, when the modeline does not exist, this brach is
		// taken when r.maxHeight >= 1.
		//
		// Show the code area near the dot.
		hCode := r.maxHeight - ui.BuffersHeight(bufMode)
		low, high := findWindow(bufCode.Dot.Line, len(bufCode.Lines), hCode)
		bufCode.TrimToLines(low, high)
	case r.maxHeight >= 2:
		// We have one line for the modeline and at least one line for the code
		// area. Note that this branch is only taken when the modeline has at
		// least one line; otherwise the previous branch always takes
		// precedence over this one.
		bufMode.TrimToLines(0, 1)
		hCode := r.maxHeight - 1
		low, high := findWindow(bufCode.Dot.Line, len(bufCode.Lines), hCode)
		bufCode.TrimToLines(low, high)
	default:
		// Height is 1 or 0. Either we really have just one line, or the
		// terminal is broken. Still try to show the current line of the code
		// area.
		bufMode = nil
		dotLine := bufCode.Dot.Line
		bufCode.TrimToLines(dotLine, dotLine+1)
	}

	var bufListing *ui.Buffer
	lister, isLister := mode.(clitypes.Lister)
	if hListing > 0 && isLister {
		bufListing = ui.Render(lister.List(hListing), buf.Width)
		// Re-render the mode line if the current mode implements
		// redrawModeLiner. This is used by completion mode where the scrollbar
		// in the mode line depends on completion.lastShown which is only known
		// after the listing has been rendered.
		//
		// We know that rendering the scrollbar never adds additional lines to
		// bufMode, we may do this without recalculating the layout. We also do
		// not need to trim bufMode because when hListing > 0, bufMode can
		// always be shown in full.
		if mode.ModeRenderFlag()&clitypes.RedrawModeLineAfterList != 0 {
			bufMode = ui.Render(mode.ModeLine(), buf.Width)
		}
	}

	// XXX The buffer contains one line in the beginning; we don't want that.
	buf.Lines = nil
	buf.Extend(bufCode, true)
	buf.Extend(bufMode, mode.ModeRenderFlag()&clitypes.CursorOnModeLine != 0)
	buf.Extend(bufListing, false)
}

// Find a window around `i` of `size`, which is smaller than `n`.
func findWindow(i, n, size int) (int, int) {
	low := i - size/2
	high := low + size
	if low < 0 {
		return 0, size
	} else if high > n {
		return n - size, n
	}
	return low, high
}

type codeContentRenderer struct {
	code    styled.Text
	dot     int
	prompt  styled.Text
	rprompt styled.Text
}

func (r *codeContentRenderer) Render(buf *ui.BufferBuilder) {
	buf.EagerWrap = true

	buf.WriteStyleds(r.prompt.ToLegacyType())
	if len(buf.Lines) == 1 && buf.Col*2 < buf.Width {
		buf.Indent = buf.Col
	}

	parts := r.code.Partition(r.dot)
	buf.WriteStyleds(parts[0].ToLegacyType())
	buf.Dot = buf.Cursor()
	buf.WriteStyleds(parts[1].ToLegacyType())

	buf.EagerWrap = false

	if rpromptWidth := styledWcswidth(r.rprompt); rpromptWidth > 0 {
		padding := buf.Width - buf.Col - rpromptWidth
		if padding >= 1 {
			buf.WriteSpaces(padding, "")
			buf.WriteStyleds(r.rprompt.ToLegacyType())
		}
	}
}

func styledWcswidth(t styled.Text) int {
	w := 0
	for _, seg := range t {
		w += util.Wcswidth(seg.Text)
	}
	return w
}

type linesRenderer struct {
	lines []string
}

func (r *linesRenderer) Render(buf *ui.BufferBuilder) {
	for i, note := range r.lines {
		if i > 0 {
			buf.Newline()
		}
		buf.WriteString(note, "")
	}
}

type codeErrorsRenderer struct {
	errors []error
}

func (r *codeErrorsRenderer) Render(buf *ui.BufferBuilder) {
	for i, err := range r.errors {
		if i > 0 {
			buf.Newline()
		}
		buf.WriteString(err.Error(), "")
	}
}
