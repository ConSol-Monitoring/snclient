package main

import (
	"fmt"
	"os"

	"github.com/consol-monitoring/snclient/pkg/snclient/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
