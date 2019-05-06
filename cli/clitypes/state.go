package clitypes

import (
	"sync"

	"github.com/elves/elvish/edit/ui"
)

// State wraps RawState, providing methods for concurrency-safe access. The
// getter methods also paper over nil values to make the empty State value more
// usable. Direct field access is also allowed but must be explicitly
// synchronized.
type State struct {
	Raw   RawState
	Mutex sync.RWMutex
}

// PopForRedraw returns a copy of the raw state, and set s.Raw.Notes = nil. Used
// for retrieving the state for rendering.
func (s *State) PopForRedraw() *RawState {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	raw := s.Raw
	s.Raw.Notes = nil
	return &raw
}

// Finalize returns a finalized State, intended for use in the final redraw.
func (s *State) Finalize() *RawState {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return &RawState{
		dummyMode{}, s.Raw.Code, len(s.Raw.Code), nil, s.Raw.Notes, ui.Key{}}
}

// Mode returns the current mode.
func (s *State) Mode() Mode {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Raw.Mode
}

// SetMode sets the current mode.
func (s *State) SetMode(mode Mode) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.Raw.Mode = mode
}

// Code returns the code.
func (s *State) Code() string {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Raw.Code
}

// CodeAndDot returns the code and dot of the state.
func (s *State) CodeAndDot() (string, int) {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Raw.Code, s.Raw.Dot
}

// CodeBeforeDot returns the part of code before the dot.
func (s *State) CodeBeforeDot() string {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Raw.Code[:s.Raw.Dot]
}

// CodeAfterDot returns the part of code after the dot.
func (s *State) CodeAfterDot() string {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Raw.Code[s.Raw.Dot:]
}

// InsertAtDot inserts the given text at the dot.
func (s *State) InsertAtDot(text string) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	raw := &s.Raw
	raw.Code = raw.Code[:raw.Dot] + text + raw.Code[raw.Dot:]
	raw.Dot += len(text)
}

// AddNote adds a note.
func (s *State) AddNote(note string) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.Raw.Notes = append(s.Raw.Notes, note)
}

// BindingKey returns BindingKey from the raw state.
func (s *State) BindingKey() ui.Key {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Raw.BindingKey
}

// SetBindingKey sets BindingKey of the raw state.
func (s *State) SetBindingKey(k ui.Key) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.Raw.BindingKey = k
}

// Reset resets the internal state to an empty value.
func (s *State) Reset() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.Raw = RawState{}
}

// RawState contains all the state of the editor.
type RawState struct {
	// The current mode.
	Mode Mode
	// The current content of the input buffer.
	Code string
	// The position of the cursor, as a byte index into Code.
	Dot int
	// Pending code, if any, such as during completion.
	Pending *PendingCode
	// Notes that have been added since the last redraw.
	Notes []string

	// In bindings, the key that the binding is handling.
	BindingKey ui.Key
}

// PendingCode represents pending code, such as during completion.
type PendingCode struct {
	// Beginning index of the text area that the pending code replaces, as a
	// byte index into RawState.Code.
	Begin int
	// End index of the text area that the pending code replaces, as a byte
	// index into RawState.Code.
	End int
	// The content of the pending code.
	Text string
}
