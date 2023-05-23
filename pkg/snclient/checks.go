package snclient

var AvailableChecks = make(map[string]CheckEntry)

// CheckHandler handles a single check.
type CheckHandler interface {
	Check(snc *Agent, Args []string) (*CheckResult, error)
}

type CheckEntry struct {
	Name    string
	Handler CheckHandler
}

type Argument struct {
	key   string
	value string
}
