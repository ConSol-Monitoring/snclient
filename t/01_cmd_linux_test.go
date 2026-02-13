package main

import (
	"testing"
)

func TestLinuxChecks(t *testing.T) {
	bin, _, cleanup := localInit(t, "")
	defer cleanup()

	// multiple filter are combined with an OR
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_drivesize", "filter='drive = /'", "filter='drive = c:'", "warn=used>100%", "crit=used>100%", "-vvv"},
		Like: []string{"OK - All 1 drive", "drive = / or drive = c:"},
		Exit: 0,
	})

	// default filter is extended with an AND
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: []string{"run", "check_drivesize", "filter+='drive = /'", "filter='drive = / and fstype = overlay'", "warn=used>100%", "crit=used>100%", "-vvv"},
		Like: []string{"OK - All 1 drive", `\(fstype not in.* and drive = /\) or drive = / and fstype = overlay`},
		Exit: 0,
	})
}
