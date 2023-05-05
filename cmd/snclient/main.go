package main

import (
	"os"

	snclient "github.com/consol-monitoring/snclient"
)

// Build contains the current git commit id
// compile passing -ldflags "-X main.Build <build sha1>" to set the id.
var Build string

// Revision contains the minor version number (number of commits)
// compile passing -ldflags "-X main.Revision <commits>" to set the revsion number.
var Revision string

func main() {
	if Revision == "" {
		Revision = "0"
	}
	if Build == "" {
		Build = "unknown"
	}

	osSignalChannel := make(chan os.Signal, 1)
	snclient.SNClient(Build, Revision, os.Args[1:], osSignalChannel)
}
