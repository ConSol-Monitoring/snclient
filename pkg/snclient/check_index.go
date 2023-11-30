package snclient

import (
	"context"
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
		name:         "check_index",
		description:  "returns list of known checks.",
		implemented:  ALL,
		detailSyntax: "${name}",
		topSyntax:    "${list}",
		listCombine:  "\n",
		emptyState:   3,
		emptySyntax:  "no checks found",
		attributes: []CheckAttribute{
			{name: "name", description: "name of the check"},
			{name: "description", description: "description of the check"},
		},
	}
}

func (l *CheckIndex) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	keys := make([]string, 0, len(AvailableChecks))

	for k := range AvailableChecks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		handler := AvailableChecks[k].Handler()
		chk := handler.Build()
		check.listData = append(check.listData, map[string]string{
			"name":        k,
			"description": chk.description,
		})
	}

	return check.Finalize()
}
