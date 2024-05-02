package snclient

import "context"

// AvailableChecks contains all registered check handler
var AvailableChecks = make(map[string]CheckEntry)

// CheckHandler handles a single check.
type CheckHandler interface {
	Check(ctx context.Context, snc *Agent, check *CheckData, args []Argument) (*CheckResult, error)
	Build() *CheckData
}

type CheckEntry struct {
	Name    string
	Handler func() CheckHandler
}

// generic key/value storage type
type Argument struct {
	key   string
	value string
	raw   string
}
