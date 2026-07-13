package snclient

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

func (snc *Agent) makeCmd(ctx context.Context, command string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command) // #nosec G204
	// prevent child from receiving signals meant for the agent only
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:     true,
		Pgid:        0,
		AmbientCaps: []uintptr{}, // do not inherit ambient capabilities by default
	}

	// add scripts path to PATH env
	scriptsPath, _ := snc.config.Section("/paths").GetString("scripts")
	cmd.Env = append(os.Environ(), "PATH="+scriptsPath+":"+os.Getenv("PATH"))

	return cmd, nil
}
