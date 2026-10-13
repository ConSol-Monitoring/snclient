package snclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invTestNoRaw mimics check_service on a Windows SCM failure: it returns a
// non-nil result with a nil error but never sets Raw (no Finalize call).
type invTestNoRaw struct{}

func (i *invTestNoRaw) Build() *CheckData {
	return &CheckData{
		name:         "check_invtestnoraw",
		implemented:  ALL,
		hasInventory: ListInventory,
		result:       &CheckResult{State: CheckExitOK},
	}
}

func (i *invTestNoRaw) Check(_ context.Context, _ *Agent, _ *CheckData, _ []Argument) (*CheckResult, error) {
	return &CheckResult{State: CheckExitUnknown, Output: "Failed to open service handler: simulated SCM failure"}, nil
}

// invTestPanic panics while collecting, to prove InvCache.Get keeps the panic
// recoverable instead of turning it into a fatal "sync: unlock of unlocked mutex".
type invTestPanic struct{}

func (i *invTestPanic) Build() *CheckData {
	return &CheckData{
		name:         "check_a000invtestpanic",
		implemented:  ALL,
		hasInventory: ListInventory,
		result:       &CheckResult{State: CheckExitOK},
	}
}

func (i *invTestPanic) Check(_ context.Context, _ *Agent, _ *CheckData, _ []Argument) (*CheckResult, error) {
	panic("simulated collection panic")
}

func TestBuildInventorySkipsResultWithoutRaw(t *testing.T) {
	snc := StartTestAgent(t, "")

	const name = "check_invtestnoraw"
	AvailableChecks[name] = CheckEntry{Name: name, Handler: func() CheckHandler { return &invTestNoRaw{} }}
	defer delete(AvailableChecks, name)

	var inv *Inventory
	assert.NotPanics(t, func() {
		inv = snc.buildInventory(t.Context(), []string{"invtestnoraw"})
	})
	require.NotNil(t, inv)
	_, present := (*inv)["invtestnoraw"]
	assert.False(t, present, "dataless result must be skipped, not dereferenced")

	StopTestAgent(t, snc)
}

func TestInvCacheGetKeepsCollectionPanicRecoverable(t *testing.T) {
	snc := StartTestAgent(t, "")

	const name = "check_a000invtestpanic"
	AvailableChecks[name] = CheckEntry{Name: name, Handler: func() CheckHandler { return &invTestPanic{} }}
	defer delete(AvailableChecks, name)

	cache := snc.invCache

	// A panic during collection must stay a recoverable panic. On the old
	// code the deferred Unlock ran on an already-unlocked mutex, an
	// unrecoverable "sync: unlock of unlocked mutex" that kills the process.
	assert.PanicsWithValue(t, "simulated collection panic", func() {
		cache.Get(t.Context(), snc)
	})

	cache.mutex.Lock()
	updating := cache.updating
	cache.mutex.Unlock()
	assert.False(t, updating, "cache must not be left stuck updating")

	StopTestAgent(t, snc)
}
