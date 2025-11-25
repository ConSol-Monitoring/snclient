package dump

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
)

// Dump displays arbitrary data.
func Dump(data any) {
	spew.Config.Indent = "\t"
	spew.Config.MaxDepth = 20
	spew.Config.DisableMethods = true
	spew.Config.SortKeys = true
	fmt.Fprintf(os.Stderr, "%s", spew.Sdump(data))
}

var dumpfile *os.File

func File(data any) {
	if dumpfile == nil {
		f, err := os.CreateTemp("", "dumpfile")
		if err != nil {
			panic(err.Error())
		}
		dumpfile = f
	}
	spew.Config.Indent = "\t"
	spew.Config.MaxDepth = 20
	spew.Config.DisableMethods = true
	spew.Config.SortKeys = true
	fmt.Fprintf(dumpfile, "%s", spew.Sdump(data))
}
