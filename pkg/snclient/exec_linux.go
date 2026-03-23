//go:build linux

package snclient

import (
	"context"
	"os"
	"syscall"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"golang.org/x/sys/unix"
)

// execCommandAsRoot runs the command with UID 0 / GID 0, leveraging capabilities
// if the process possesses CAP_SETUID and CAP_SETGID. If already root, it works identically.
func (snc *Agent) execCommandAsRoot(ctx context.Context, command string, timeout int64) (stdout, stderr string, exitCode int64, err error) {
	cmd, err := snc.MakeCmd(ctx, command)
	if err == nil {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: 0, Gid: 0}
		stdout, stderr, exitCode, _, err = snc.runExternalCommand(ctx, cmd, timeout)
	}

	return stdout, stderr, exitCode, err
}

// HasCapabilities returns true if the process possesses CAP_SETUID and CAP_SETGID.
func (snc *Agent) HasCapabilities() bool {
	header := unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     convert.Int32(os.Getpid()),
	}
	var data [2]unix.CapUserData
	err := unix.Capget(&header, &data[0])
	if err != nil {
		return false
	}

	return (data[0].Effective&(1<<unix.CAP_SETUID) != 0) && (data[0].Effective&(1<<unix.CAP_SETGID) != 0)
}
