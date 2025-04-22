package snclient

import (
	"context"
)

func init() {
	AvailableChecks["check_pdh"] = CheckEntry{"check_pdh", NewCheckPDH}
}

type CheckPDH struct {
	CounterPath          string
	HostName             string
	Type                 string
	Instances            bool
	ExpandIndex          bool
	EnglishFallBackNames bool
	OptionalAlias        string
}

func NewCheckPDH() CheckHandler {
	return &CheckPDH{}
}

func (c *CheckPDH) Build() *CheckData {
	return &CheckData{
		implemented:  Windows,
		name:         "check_pdh",
		description:  "Checks pdh paths and handles wildcard expansion. Also available with the alias CheckCounter",
		detailSyntax: "%(name)",
		okSyntax:     "%(status) - All %(count) counter values are ok",
		topSyntax:    "%(status) - %(problem_count)/%(count) counter (%(count)) %(problem_list)",
		emptySyntax:  "%(status) - No counter found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"counter":      {value: &c.CounterPath, description: "The fully qualified Counter Name"},
			"Counter":      {value: &c.CounterPath, description: "The fully qualified Counter Name"},
			"host":         {value: &c.HostName, description: "The Name Of the Host Mashine in Network where the Counter uld be searched, defults to local mashine"},
			"expand-index": {value: &c.ExpandIndex, description: "Should Indices be translated?"},
			"instances":    {value: &c.Instances, description: "Expand WildCards And Fethch all instances"},
			"type":         {value: &c.Type, description: "this can be large or float depending what you expect, default is large "},
			"english":      {value: &c.EnglishFallBackNames, description: "Using English Names Regardless of system Language requires Windows Vista or higher"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "value ", description: "The counter value (either float or int)"},
		},
		exampleDefault: `
		check_pdh "counter=foo" "warn=value > 80" "crit=value > 90"
		Everything looks good
		'foo value'=18;80;90
		`,
		exampleArgs:     `counter=\\System\\System Up Time" "warn=value > 5" "crit=value > 9999`,
		argsPassthrough: true,
	}
}

// Check implements CheckHandler.
func (c *CheckPDH) Check(ctx context.Context, snc *Agent, check *CheckData, args []Argument) (*CheckResult, error) {
	return c.check(ctx, snc, check, args)
}
