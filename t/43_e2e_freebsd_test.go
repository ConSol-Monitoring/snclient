package main

import (
	"os"
	"testing"
)

const (
	localFreeBSDINIPath = `/etc/snclient/snclient_local.ini`
)

func TestFreeBSD(t *testing.T) {
	// skip test unless requested, it will uninstall existing installations
	if os.Getenv("SNCLIENT_INSTALL_TEST") != "1" {
		t.Skipf("SKIPPED: pkg installer test requires env SNCLIENT_INSTALL_TEST=1")

		return
	}

	t.Logf("not yet implemented...")
	bin := getBinary()
	writeFile(t, localFreeBSDINIPath, localTestINI)
	waitUntilResponse(t, bin)
}
