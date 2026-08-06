//go:build windows

package snclient

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMountWindowsNoDuplicateEntries makes sure a drive that is reported by both
// the partition discovery (getDrives) and
// the volume discovery (getVolumes)
// is only listed once in the output.
func TestMountWindowsNoDuplicateEntries(t *testing.T) {
	snc := StartTestAgent(t, "")

	// force every entry into the problem list so all mounts end up in the output
	res := snc.RunCheck("check_mount", []string{"options=force-mount-listing", "detail-syntax=mount=${mount}"})
	assert.Equalf(t, CheckExitWarning, res.State, "state Warning")
	output := string(res.BuildPluginOutput())

	mountRe := regexp.MustCompile(`mount (\S+)`)
	mountSeen := map[string]int{}
	for _, match := range mountRe.FindAllStringSubmatch(output, -1) {
		mountSeen[match[1]]++
	}

	require.NotEmptyf(t, mountSeen, "expected at least one mount in output")

	for mount, count := range mountSeen {
		assert.Equalf(t, 1, count, "mount %s reported %d times, expected exactly once", mount, count)
	}

	StopTestAgent(t, snc)
}
