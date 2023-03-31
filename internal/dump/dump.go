package dump

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
)

// Dump displays arbitrary data.
func Dump(v interface{}) {
	spew.Config.Indent = "\t"
	spew.Config.MaxDepth = 20
	spew.Config.DisableMethods = true
	spew.Config.SortKeys = true
	fmt.Fprintf(os.Stderr, "%s", spew.Sdump(v))
}
