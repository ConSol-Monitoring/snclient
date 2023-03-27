package snclient

// CheckHandler handles a single check.
type CheckHandler interface {
	Check(Args []string) (*CheckResult, error)
}
