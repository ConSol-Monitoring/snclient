package snclient

import (
	"sort"
	"strings"
)

func init() {
	AvailableChecks["check_index"] = CheckEntry{"check_index", new(CheckIndex)}
}

type CheckIndex struct {
	noCopy noCopy
}

func (l *CheckIndex) Check(_ *Agent, _ []string) (*CheckResult, error) {
	state := int64(0)

	keys := make([]string, 0, len(AvailableChecks))

	for k := range AvailableChecks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	output := strings.Join(keys, "\n")

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}
