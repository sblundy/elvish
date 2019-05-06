package histutil

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewFuser(t *testing.T) {
	mockError := errors.New("mock error")
	_, err := NewFuser(&testDB{oneOffError: mockError})
	if err != mockError {
		t.Errorf("NewFuser -> error %v, want %v", err, mockError)
	}
}

var fuserStore = &testDB{cmds: []string{"store 1"}}

func TestFuser(t *testing.T) {
	f, err := NewFuser(fuserStore)
	if err != nil {
		t.Errorf("NewFuser -> error %v, want nil", err)
	}

	// AddCmd should not add command to session history if backend has an error
	// adding the command.
	mockError := errors.New("mock error")
	fuserStore.oneOffError = mockError
	_, err = f.AddCmd("haha")
	if err != mockError {
		t.Errorf("AddCmd doesn't forward backend error")
	}
	if len(f.SessionCmds()) != 0 {
		t.Errorf("AddCmd adds command to session history when backend errors")
	}

	// AddCmd should add command to both storage and session
	f.AddCmd("session 1")
	if !reflect.DeepEqual(fuserStore.cmds, []string{"store 1", "session 1"}) {
		t.Errorf("AddCmd doesn't add command to backend storage")
	}
	if !reflect.DeepEqual(f.SessionCmds(), []Entry{{"session 1", 1}}) {
		t.Errorf("AddCmd doesn't add command to session history")
	}

	// AllCmds should return all commands from the storage when the Fuser was
	// created followed by session commands
	fuserStore.AddCmd("other session 1")
	fuserStore.AddCmd("other session 2")
	f.AddCmd("session 2")
	cmds, err := f.AllCmds()
	if err != nil {
		t.Errorf("AllCmds returns error")
	}
	if !reflect.DeepEqual(cmds, []Entry{
		{"store 1", 0}, {"session 1", 1}, {"session 2", 4}}) {
		t.Errorf("AllCmds doesn't return all commands")
	}

	// AllCmds should forward backend storage error
	mockError = errors.New("another mock error")
	fuserStore.oneOffError = mockError
	_, err = f.AllCmds()
	if err != mockError {
		t.Errorf("AllCmds doesn't forward backend error")
	}

	// Walker should return a walker that walks through all commands
	w := f.Walker("")
	w.Prev()
	checkWalkerCurrent(t, w, 4, "session 2")
	w.Prev()
	checkWalkerCurrent(t, w, 1, "session 1")
	w.Prev()
	checkWalkerCurrent(t, w, 0, "store 1")
	checkError(t, w.Prev(), ErrEndOfHistory)
}
