package snclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckNTPOffsetW32TimeEntry(t *testing.T) {
	l := &CheckNTPOffset{}
	entry := l.defaultEntry("w32time")

	fillW32TimeEntry(entry, &w32timeStatus{
		LeapIndicator:      0,
		Stratum:            4,
		RootDelay:          38510100,
		RootDispersion:     28135000,
		Source:             "time.windows.com,0x8",
		PhaseOffset:        6151700,
		State:              w32timeStateSync,
		LastSyncResult:     w32timeSyncResultSuccess,
		LastSuccessfulSync: time.Date(2023, 12, 20, 9, 30, 13, 659032000, time.UTC),
	})

	assert.Equal(t, map[string]string{
		"source":                "w32time",
		"server":                "time.windows.com",
		"stratum":               "4",
		"jitter":                "",
		"offset":                "6.151700",
		"offset_seconds":        "0.006152",
		"leap_indicator":        "0",
		"state":                 "2",
		"state_name":            "Sync",
		"last_sync_result":      "0",
		"last_sync_result_name": "Success",
		"root_delay":            "38.510100",
		"root_dispersion":       "28.135000",
		"last_sync":             "1703064613.659",
	}, entry)
}

func TestCheckNTPOffsetW32TimeEntryHold(t *testing.T) {
	l := &CheckNTPOffset{}
	entry := l.defaultEntry("w32time")

	fillW32TimeEntry(entry, &w32timeStatus{
		Stratum:        4,
		Source:         "Free-running System Clock",
		PhaseOffset:    -7843300,
		State:          w32timeStateHold,
		LastSyncResult: w32timeSyncResultStaleData,
		LeapIndicator:  3,
	})

	assert.Equal(t, "w32time: State Machine: 1 (Hold)", entry["_error"])
	assert.Equal(t, "2", entry["_exit"])
	assert.Equal(t, "Free-running System Clock", entry["server"])
	assert.Equal(t, "-7.843300", entry["offset"])
	assert.Equal(t, "StaleData", entry["last_sync_result_name"])
	assert.Empty(t, entry["last_sync"])
}
