package snclient

import "context"

// AvailableChecks contains all registered check handler
var AvailableChecks = make(map[string]CheckEntry)

// CheckHandler handles a single check.
type CheckHandler interface {
	Check(ctx context.Context, snc *Agent, check *CheckData, args []Argument) (*CheckResult, error)
	Build() *CheckData
}

// CheckEntry is a named CheckHandler entry.
type CheckEntry struct {
	Name    string
	Handler func() CheckHandler
}

// Argument is a generic key/value storage type.
type Argument struct {
	key   string
	value string
	raw   string
}

// ArgumentList is a list of Argument objects.
type ArgumentList []Argument

// return list of raw values as list of strings.
func (al ArgumentList) RawList() []string {
	rawList := make([]string, 0, len(al))
	for _, arg := range al {
		rawList = append(rawList, arg.raw)
	}

	return rawList
}
