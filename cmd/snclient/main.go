package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/consol-monitoring/snclient/pkg/snclient/commands"
)

// free memory every minute
func init() {
	go func() {
		t := time.Tick(1 * time.Minute)
		for {
			<-t
			debug.FreeOSMemory()
		}
	}()
}

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
