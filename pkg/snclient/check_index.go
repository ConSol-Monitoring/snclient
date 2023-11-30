package snclient

import (
	"context"
	"sort"
	"strings"
)

func init() {
	AvailableChecks["check_index"] = CheckEntry{"check_index", NewCheckIndex}
}

type CheckIndex struct {
	noCopy noCopy
}

func NewCheckIndex() CheckHandler {
	return &CheckIndex{}
}

func (l *CheckIndex) Build() *CheckData {
	return &CheckData{
		name:        "check_index",
		description: "returns list of known checks.",
	}
}

func (l *CheckIndex) Check(_ context.Context, _ *Agent, _ *CheckData, _ []Argument) (*CheckResult, error) {
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
