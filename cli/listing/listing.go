// Package listing implements the Elvish-agnostic core of the listing mode.
//
// The listing mode is a mode that shows a list of entries, and allows the user
// to browse and filter the list, and select a specific entry from it. It is
// generic and requires a "start config" to specify the list of entries and
// certain behaviors.
//
// The editor will most likely want to have several different possible listing
// modes. For instance, the Elvish editor has a history listing mode, a location
// mode, among others. Under the hood, those different modes share the same
// underlying listing.Mode instance and are just different instantiations of the
// same mode; "starting the location mode" is just a shorthand for "starting the
// listing mode with the start config suitable for location listing".
package listing

import (
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/elves/elvish/cli/clitypes"
	"github.com/elves/elvish/edit/tty"
	"github.com/elves/elvish/edit/ui"
	"github.com/elves/elvish/styled"
)

// Mode represents a listing mode, implementing the clitypes.Mode interface.
type Mode struct {
	StartConfig
	state      State
	stateMutex sync.Mutex
}

// StartConfig is the configuration for starting the listing mode.
type StartConfig struct {
	Name        string
	KeyHandler  func(ui.Key) clitypes.HandlerAction
	ItemsGetter func(filter string) Items
	StartFilter bool
	AutoAccept  bool
	SelectLast  bool
}

// Items is an interface for accessing items to show in the listing mode.
type Items interface {
	Len() int
	Show(int) styled.Text
	Accept(int, *clitypes.State)
}

// SliceItems returns an Items consisting of the given texts.
func SliceItems(texts ...styled.Text) Items { return sliceItems{texts} }

type sliceItems struct{ texts []styled.Text }

func (it sliceItems) Len() int                    { return len(it.texts) }
func (it sliceItems) Show(i int) styled.Text      { return it.texts[i] }
func (it sliceItems) Accept(int, *clitypes.State) {}

// Start starts the listing mode, using the given config and resetting all
// states.
func (m *Mode) Start(cfg StartConfig) {
	*m = Mode{
		StartConfig: cfg,
		state: State{
			itemsGetter: cfg.ItemsGetter, selectLast: cfg.SelectLast,
			filtering: cfg.StartFilter},
	}
	m.state.refilter("")
}

// ModeLine returns a modeline showing the specified name of the mode.
func (m *Mode) ModeLine() ui.Renderer {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	return ui.NewModeLineRenderer(" "+m.Name+" ", m.state.filter)
}

// ModeRenderFlag returns CursorOnModeLine if filtering, or 0 otherwise.
func (m *Mode) ModeRenderFlag() clitypes.ModeRenderFlag {
	if m.state.filtering {
		return clitypes.CursorOnModeLine
	}
	return 0
}

// HandleEvent handles key events and ignores other types of events.
func (m *Mode) HandleEvent(e tty.Event, st *clitypes.State) clitypes.HandlerAction {
	switch e := e.(type) {
	case tty.KeyEvent:
		if m.KeyHandler == nil {
			m.stateMutex.Lock()
			defer m.stateMutex.Unlock()
			return defaultBinding(ui.Key(e), st, &m.state)
		}
		return m.KeyHandler(ui.Key(e))
	default:
		return clitypes.NoAction
	}
}

// DefaultHandler handles keys when filtering, and resets the mode when not.
func (m *Mode) DefaultHandler(st *clitypes.State) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	defaultHandler(st.BindingKey(), st, &m.state)
	if m.AutoAccept && m.state.items.Len() == 1 {
		m.state.items.Accept(0, st)
		st.SetMode(nil)
	}
}

func defaultBinding(k ui.Key, st *clitypes.State, mst *State) clitypes.HandlerAction {
	switch k {
	case ui.K('[', ui.Ctrl):
		// TODO(xiaq): Go back to previous mode instead of the initial mode.
		st.SetMode(nil)
	case ui.K(ui.Down):
		mst.Down()
	case ui.K(ui.Up):
		mst.Up()
	case ui.K(ui.Tab):
		mst.DownCycle()
	case ui.K(ui.Tab, ui.Shift):
		mst.UpCycle()
	case ui.K('F', ui.Ctrl):
		mst.ToggleFiltering()
	default:
		return defaultHandler(k, st, mst)
	}
	return 0
}

func defaultHandler(k ui.Key, st *clitypes.State, mst *State) clitypes.HandlerAction {
	if mst.filtering {
		filter := mst.filter
		if k == ui.K(ui.Backspace) {
			_, size := utf8.DecodeLastRuneInString(filter)
			if size > 0 {
				mst.refilter(filter[:len(filter)-size])
			}
		} else if likeChar(k) {
			mst.refilter(mst.filter + string(k.Rune))
		} else {
			st.AddNote("Unbound: " + k.String())
		}
		return 0
	}
	st.SetMode(nil)
	// TODO: Return ReprocessEvent
	return 0
}

func likeChar(k ui.Key) bool {
	return k.Mod == 0 && k.Rune > 0 && unicode.IsGraphic(k.Rune)
}

// MutateStates mutates the states using the given function, guarding the
// mutation with the mutex.
func (m *Mode) MutateStates(f func(*State)) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	f(&m.state)
}

// AcceptItem accepts the currently selected item.
func (m *Mode) AcceptItem(st *clitypes.State) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.state.items.Accept(m.state.selected, st)
}

// AcceptItemAndClose accepts the currently selected item and closes the listing
// mode.
func (m *Mode) AcceptItemAndClose(st *clitypes.State) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	m.state.items.Accept(m.state.selected, st)
	st.SetMode(nil)
}

// The number of lines the listing mode keeps between the current selected item
// and the top and bottom edges of the window, unless the available height is
// too small or if the selected item is near the top or bottom of the list.
var respectDistance = 2

var (
	styleForSelected = "inverse"
	styleForLastLine = "underlined"
)

// List renders the listing.
func (m *Mode) List(maxHeight int) ui.Renderer {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	st := &m.state

	n := st.items.Len()
	if n == 0 {
		// No result.
		return ui.NewStringRenderer("(no result)")
	}

	newFirst, firstCrop := findWindow(st.items, st.first, st.selected, maxHeight)
	st.first = newFirst

	var allLines []styled.Text
	upper := n
	lastCropped := false
	for i := st.first; i < n; i++ {
		lines := st.items.Show(i).SplitByRune('\n')
		if i == st.first && firstCrop > 0 {
			lines = lines[firstCrop:]
		}
		if i == st.selected {
			for i := range lines {
				lines[i] = styled.Transform(lines[i], styleForSelected)
			}
		}
		// TODO: Optionally, add underlines to the last line as separators
		// between adjacent entries.
		if len(allLines)+len(lines) > maxHeight {
			lines = lines[:len(allLines)+len(lines)-maxHeight]
			lastCropped = true
		}
		allLines = append(allLines, lines...)
		if len(allLines) >= maxHeight {
			upper = i + 1
			break
		}
	}

	rd := NewStyledTextsRenderer(allLines)
	if st.first > 0 || firstCrop > 0 || upper < n || lastCropped {
		rd = ui.NewRendererWithVerticalScrollbar(rd, n, st.first, upper)
	}
	return rd
}

// Determines the index of the first item to show in listing.
func findWindow(items Items, oldFirst, selected, maxHeight int) (first, crop int) {
	n := items.Len()
	selectedHeight := items.Show(selected).CountLines()

	if maxHeight <= selectedHeight {
		// The height is not big enough (or just big enough) to fit the selected
		// item. Fit as much as the selected item as we can.
		return selected, 0
	}

	// Determine the initial budgets for expanding on both directions.
	budget := maxHeight - selectedHeight
	budgetUp := 0
	if budget >= 2*respectDistance {
		// If the height is big enough to maintain the respect distances on
		// both sides, we start with a budget that leaves just enough
		// respect distance for the downward side.
		budgetUp = budget - respectDistance
	} else {
		// Otherwise we split the budgets for the two directions in half.
		budgetUp = budget / 2
	}

	budgetDown := budget - budgetUp
	// Calculate the most amount of the budget we can consume by expanding
	// downwards. The result will be used for two purposes, 1) determining
	// whether to reallocate some of budgetDown to budgetUp, and 2) determining
	// when to stop expanding upwards.
	useDown := 0
	for i := selected + 1; i < n; i++ {
		useDown += items.Show(i).CountLines()
		if useDown >= budget {
			break
		}
	}
	if budgetDown > useDown {
		// We reached item n-1 without using up budgetDown. Reallocate the extra
		// budget towards budgetUp.
		budgetUp += budgetDown - useDown
	}

	useUp := 0
	// Extend upwards until any of the following:
	// * We have exhausted budgetUp;
	// * We have reached oldFirst, achieved the upward respect distance, and
	//   will be able to use up the entire budget when expanding downwards
	//   later;
	// * We have reached item 0.
	for i := selected - 1; i >= 0; i-- {
		useUp += items.Show(i).CountLines()
		if useUp >= budgetUp {
			return i, useUp - budgetUp
		}
		if i <= oldFirst && useUp >= respectDistance && useUp+useDown >= budget {
			return i, 0
		}
	}
	return 0, 0
}
