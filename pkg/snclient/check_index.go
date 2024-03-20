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
			{name: "alias", description: "check is an alias: 0 / 1"},
			{name: "script", description: "check is a (wrapped) script: 0 / 1"},
		},
		exampleDefault: `
    check_index filter="implemented = 1"
    check_cpu...
	`,
	}
}

func (l *CheckIndex) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	keys := make([]string, 0)
	for k := range AvailableChecks {
		keys = append(keys, k)
	}
	for k := range snc.runSet.cmdAliases {
		keys = append(keys, k)
	}
	for k := range snc.runSet.cmdWraps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	alreadyShown := map[string]bool{}

	bool2Str := map[bool]string{false: "0", true: "1"}
	for _, name := range keys {
		if alreadyShown[name] {
			continue
		}
		alreadyShown[name] = true
		entry := map[string]string{
			"name":        name,
			"description": "",
			"implemented": "0",
			"windows":     "0",
			"linux":       "0",
			"osx":         "0",
			"freebsd":     "0",
			"alias":       "0",
			"script":      "0",
		}
		if _, ok := snc.runSet.cmdAliases[name]; ok {
			entry["description"] = "command alias"
			entry["alias"] = "1"
			entry["implemented"] = "1"
			entry[runtime.GOOS] = "1"
		} else if _, ok := snc.runSet.cmdWraps[name]; ok {
			entry["description"] = "custom script"
			entry["script"] = "1"
			entry["implemented"] = "1"
			entry[runtime.GOOS] = "1"
		} else if e, ok := AvailableChecks[name]; ok {
			handler := e.Handler()
			chk := handler.Build()
			entry["description"] = chk.description
			entry["implemented"] = bool2Str[chk.isImplemented(runtime.GOOS)]
			entry["windows"] = bool2Str[chk.isImplemented("windows")]
			entry["linux"] = bool2Str[chk.isImplemented("linux")]
			entry["osx"] = bool2Str[chk.isImplemented("darwin")]
			entry["freebsd"] = bool2Str[chk.isImplemented("freebsd")]
		}

		check.listData = append(check.listData, entry)
	}

	return check.Finalize()
}
