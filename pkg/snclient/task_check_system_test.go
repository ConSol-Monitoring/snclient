package snclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestForgetUnseenStorageDevices(t *testing.T) {
	now := time.Now()
	cutoff := now.Add(-storageDeviceForgetDuration)

	seen := map[string]time.Time{
		"sda":  now,
		"sdb":  now.Add(-storageDeviceForgetDuration - time.Second),
		"nvme": now,
	}

	watch := []string{"sda", "sdb", "nvme", "untracked"}
	kept := forgetUnseenStorageDevices(watch, seen, cutoff)

	assert.Equal(t, []string{"sda", "nvme"}, kept)
	assert.NotContains(t, seen, "sdb", "forgotten device removed from seen map")
	assert.NotContains(t, seen, "untracked", "untracked device removed from seen map")
	assert.Contains(t, seen, "sda", "still-watched device kept in seen map")
}
