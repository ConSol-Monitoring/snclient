package snclient

import (
	"os/exec"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// not supported on windows
}
