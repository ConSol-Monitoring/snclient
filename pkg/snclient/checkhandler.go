package snclient

// CheckHandler handles a single check.
type CheckHandler interface {
	Check(snc *Agent, Args []string) (*CheckResult, error)
}
