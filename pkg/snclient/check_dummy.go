package snclient

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func init() {
	AvailableChecks["check_dummy"] = CheckEntry{"check_dummy", new(CheckDummy)}
}

type CheckDummy struct {
	noCopy noCopy
}

func (l *CheckDummy) Build() *CheckData {
	return &CheckData{
		name:            "check_dummy",
		description:     "This check simply sets the state to the given value and outputs the remaining arguments.",
		argsPassthrough: true,
	}
}

func (l *CheckDummy) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	state := int64(0)
	output := "Dummy Check"

	if len(check.rawArgs) > 0 {
		res, err := strconv.ParseInt(check.rawArgs[0], 10, 64)
		if err != nil {
			res = CheckExitUnknown
			output = fmt.Sprintf("cannot parse state to int: %s", err)
		}

		state = res
	}

	if len(check.rawArgs) > 1 {
		output = strings.Join(check.rawArgs[0:], " ")
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}
