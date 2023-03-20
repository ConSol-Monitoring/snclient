package main

import (
	snclient "github.com/sni/snclient"
)

// Build contains the current git commit id
// compile passing -ldflags "-X main.Build <build sha1>" to set the id.
var Build string

func main() {
	snclient.Daemon(Build)
}
