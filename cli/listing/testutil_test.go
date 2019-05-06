package listing

import (
	"testing"

	"github.com/elves/elvish/styled"
	"github.com/elves/elvish/tt"
)

func TestMatchItems(t *testing.T) {
	a := styled.Plain("a")
	b := styled.Plain("b")
	c := styled.Plain("c")
	matcher := MatchItems(a, b)
	tt.Test(t, tt.Fn("matcher.Match", matcher.Match), tt.Table{
		Args(tt.RetValue(SliceItems(a, b))).Rets(true),
		Args(tt.RetValue(SliceItems())).Rets(false),
		Args(tt.RetValue(SliceItems(a))).Rets(false),
		Args(tt.RetValue(SliceItems(c))).Rets(false),
		Args(tt.RetValue(SliceItems(a, c))).Rets(false),
	})
}
