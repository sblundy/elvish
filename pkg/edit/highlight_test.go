package edit

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/elves/elvish/pkg/cli/term"
	"github.com/elves/elvish/pkg/env"
	"github.com/elves/elvish/pkg/eval"
	"github.com/elves/elvish/pkg/eval/vars"
	"github.com/elves/elvish/pkg/parse"
	"github.com/elves/elvish/pkg/testutil"
	"github.com/elves/elvish/pkg/tt"
)

// High-level sanity test.

func TestHighlighter(t *testing.T) {
	f := setup()
	defer f.Cleanup()

	feedInput(f.TTYCtrl, "put $true")
	f.TestTTY(t,
		"~> put $true", Styles,
		"   vvv $$$$$", term.DotHere,
	)

	feedInput(f.TTYCtrl, "x")
	f.TestTTY(t,
		"~> put $truex", Styles,
		"   vvv ??????", term.DotHere, "\n",
		"compilation error: 4-10 in [tty]: variable $truex not found",
	)
}

// Fine-grained tests against the highlighter.

func TestCheck(t *testing.T) {
	ev := eval.NewEvaler()
	ev.Global = eval.NsBuilder{"good": vars.FromInit(0)}.Ns()

	tt.Test(t, tt.Fn("check", check), tt.Table{
		tt.Args(ev, mustParse("")).Rets(noError),
		tt.Args(ev, mustParse("echo $good")).Rets(noError),
		// TODO: Check the range of the returned error
		tt.Args(ev, mustParse("echo $bad")).Rets(anyError),
	})
}

type anyErrorMatcher struct{}

func (anyErrorMatcher) Match(ret tt.RetValue) bool {
	err, _ := ret.(error)
	return err != nil
}

var (
	noError  = error(nil)
	anyError anyErrorMatcher
)

const colonInFilenameOk = runtime.GOOS != "windows"

func TestMakeHasCommand(t *testing.T) {
	ev := eval.NewEvaler()

	// Set up global functions and modules in the evaler.
	goodFn := eval.NewGoFn("good", func() {})
	ev.Global = eval.NsBuilder{}.
		AddFn("good", goodFn).
		AddNs("a",
			eval.NsBuilder{}.
				AddFn("good", goodFn).
				AddNs("b", eval.NsBuilder{}.AddFn("good", goodFn).Ns()).
				Ns()).
		Ns()

	// Set up environment.
	testDir, cleanup := testutil.InTestDir()
	defer cleanup()
	oldPath := os.Getenv(env.PATH)
	defer os.Setenv(env.PATH, oldPath)
	if runtime.GOOS == "windows" {
		oldPathExt := os.Getenv(env.PATHEXT)
		defer os.Setenv(env.PATHEXT, oldPathExt)
		os.Unsetenv(env.PATHEXT) // force default value
	}

	// Set up a directory in PATH.
	os.Setenv(env.PATH, filepath.Join(testDir, "bin"))
	mustMkdirAll("bin")
	mustMkExecutable("bin/external")
	mustMkExecutable("bin/@external")
	if colonInFilenameOk {
		mustMkExecutable("bin/ex:tern:al")
	}

	// Set up a directory not in PATH.
	mustMkdirAll("a/b/c")
	mustMkExecutable("a/b/c/executable")

	tt.Test(t, tt.Fn("hasCommand", hasCommand), tt.Table{
		// Builtin special form
		tt.Args(ev, "if").Rets(true),
		// Builtin function
		tt.Args(ev, "put").Rets(true),
		// User-defined function
		tt.Args(ev, "good").Rets(true),
		// Function in modules
		tt.Args(ev, "a:good").Rets(true),
		tt.Args(ev, "a:b:good").Rets(true),

		// Non-searching directory and external
		tt.Args(ev, "./a").Rets(true),
		tt.Args(ev, "a/b").Rets(true),
		tt.Args(ev, "a/b/c/executable").Rets(true),

		// External in PATH
		tt.Args(ev, "external").Rets(true),
		tt.Args(ev, "@external").Rets(true),
		tt.Args(ev, "ex:tern:al").Rets(colonInFilenameOk),

		// Non-existent
		tt.Args(ev, "bad").Rets(false),
		tt.Args(ev, "a:").Rets(false),
		tt.Args(ev, "a:bad").Rets(false),
		tt.Args(ev, "a:b:bad").Rets(false),
		tt.Args(ev, "./bad").Rets(false),
		tt.Args(ev, "a/bad").Rets(false),
	})
}

func mustParse(src string) parse.Tree {
	tree, err := parse.Parse(parse.SourceForTest(src))
	if err != nil {
		panic(err)
	}
	return tree
}

func mustMkdirAll(path string) {
	err := os.MkdirAll(path, 0700)
	if err != nil {
		panic(err)
	}
}

func mustMkExecutable(path string) {
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	err := ioutil.WriteFile(path, nil, 0700)
	if err != nil {
		panic(err)
	}
}
