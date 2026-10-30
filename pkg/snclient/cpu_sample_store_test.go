package snclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCPUSampleStore(t *testing.T) {
	now := time.Now()
	store := newCPUSampleStore()

	// first sample: no baseline yet
	pct, ok := store.update(1, 1000, 10, now)
	assert.False(t, ok, "no baseline on first sample")
	assert.Zero(t, pct)

	// second sample less than a minute later: still no baseline, not stored (max 1 sample per minute)
	pct, ok = store.update(1, 1000, 12, now.Add(30*time.Second))
	assert.False(t, ok, "no baseline while samples are younger than a minute")
	assert.Zero(t, pct)

	// baseline available once the first sample is at least one minute old
	pct, ok = store.update(1, 1000, 15, now.Add(60*time.Second))
	assert.True(t, ok, "baseline available after one minute")
	assert.InDelta(t, 100.0*5/60, pct, 0.001)

	// the 60s sample is not yet a minute old, so the first sample is still the baseline
	pct, ok = store.update(1, 1000, 20, now.Add(90*time.Second))
	assert.True(t, ok, "baseline available")
	assert.InDelta(t, 100.0*10/90, pct, 0.001)

	// the most recent previous sample that is at least a minute old becomes the new baseline
	pct, ok = store.update(1, 1000, 25, now.Add(150*time.Second))
	assert.True(t, ok, "baseline available")
	assert.InDelta(t, 100.0*10/90, pct, 0.001)

	// a new process (pid reuse / new process) has no baseline
	pct, ok = store.update(2, 2000, 10, now.Add(150*time.Second))
	assert.False(t, ok, "no baseline for new process")
	assert.Zero(t, pct)

	// samples expire after 60 minutes
	exp := newCPUSampleStore()
	exp.update(1, 1000, 10, now)
	exp.update(1, 1000, 20, now.Add(2*time.Minute))
	pct, ok = exp.update(1, 1000, 120, now.Add(65*time.Minute))
	assert.False(t, ok, "baseline expired after 60 minutes")
	assert.Zero(t, pct)
}

func TestAgentCPUUsageSinceNilSafe(t *testing.T) {
	_, ok := (*Agent)(nil).cpuUsageSince(1, 1000, 10, time.Now())
	assert.False(t, ok, "nil agent must not panic and reports no baseline")

	snc := NewAgentSimple(&AgentFlags{})
	_, ok = snc.cpuUsageSince(1, 1000, 10, time.Now())
	require.False(t, ok, "no baseline on first sample")
}
