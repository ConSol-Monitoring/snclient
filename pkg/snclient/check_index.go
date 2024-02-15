package snclient

import (
	"context"
	"runtime"
	"sort"
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
		name:          "check_index",
		description:   "returns list of known checks.",
		implemented:   ALL,
		detailSyntax:  "${name}",
		topSyntax:     "${list}",
		listCombine:   "\n",
		defaultFilter: "implemented = 1",
		emptyState:    3,
		emptySyntax:   "no checks found",
		attributes: []CheckAttribute{
			{name: "name", description: "name of the check"},
			{name: "description", description: "description of the check"},
			{name: "implemented", description: "check is available on current platform: 0 / 1"},
			{name: "windows", description: "check is available on windows: 0 / 1"},
			{name: "linux", description: "check is available on linux: 0 / 1"},
			{name: "osx", description: "check is available on mac osx: 0 / 1"},
			{name: "freebsd", description: "check is available on freebsd: 0 / 1"},
		},
		exampleDefault: `
    check_index filter="implemented = 1"
    check_cpu...
	`,
	}
}

func (l *CheckIndex) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	keys := make([]string, 0, len(AvailableChecks))

	for k := range AvailableChecks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	bool2Str := map[bool]string{false: "0", true: "1"}
	for _, k := range keys {
		handler := AvailableChecks[k].Handler()
		chk := handler.Build()
		check.listData = append(check.listData, map[string]string{
			"name":        k,
			"description": chk.description,
			"implemented": bool2Str[chk.isImplemented(runtime.GOOS)],
			"windows":     bool2Str[chk.isImplemented("windows")],
			"linux":       bool2Str[chk.isImplemented("linux")],
			"osx":         bool2Str[chk.isImplemented("darwin")],
			"freebsd":     bool2Str[chk.isImplemented("freebsd")],
		})
	}

	return check.Finalize()
}
