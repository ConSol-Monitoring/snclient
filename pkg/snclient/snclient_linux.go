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
