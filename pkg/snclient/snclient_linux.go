package snclient

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func init() {
	if HasCapabilities() {
		err := clearInheritableCaps()
		if err != nil {
			panic("failed to drop capabilities: " + err.Error())
		}
	}
}

// clear all capabilities from the inheritable set, so that child processes do
// not inherit them. This is important for security, as it prevents unprivileged
// child processes from gaining elevated privileges.
func clearInheritableCaps() error {
	hdr := unix.CapUserHeader{
		Version: unix.LINUX_CAPABILITY_VERSION_3,
		Pid:     0, // current process
	}

	// Version 3 supports 64 capabilities split across two uint32 values.
	caps := [2]unix.CapUserData{}

	// Read current capability state.
	if err := unix.Capget(&hdr, &caps[0]); err != nil {
		return fmt.Errorf("capget: %w", err)
	}

	// Clear only the inheritable set.
	caps[0].Inheritable = 0
	caps[1].Inheritable = 0

	// Write capabilities back.
	if err := unix.Capset(&hdr, &caps[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}

	return nil
}

// add cap_setuid and cap_setgid to inheritable set
func prepareCapsForExec() error {
	if !HasCapabilities() {
		return nil
	}

	caps := []int{unix.CAP_SETUID, unix.CAP_SETGID}

	// 1. Read current caps
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return fmt.Errorf("capget: %w", err)
	}

	// 2. Add both to the Inheritable set
	for _, c := range caps {
		idx, mask := uint(c)/32, uint32(1)<<(uint(c)%32)
		data[idx].Inheritable |= mask
	}
	if err := unix.Capset(&hdr, &data[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}

	// 3. Raise each into the Ambient set
	for _, c := range caps {
		if err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_RAISE, uintptr(c), 0, 0); err != nil {
			return fmt.Errorf("prctl: %w", err)
		}
	}

	return nil
}
