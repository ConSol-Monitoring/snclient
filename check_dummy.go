package snclient

import (
	"fmt"
	"strconv"
	"strings"
)

func init() {
	AvailableChecks["CheckDummy"] = CheckEntry{"CheckDummy", new(CheckDummy)}
}

type CheckDummy struct {
	noCopy noCopy
}

/* CheckDummy <state> <text>
 * This check simply sets the state to the given value and outputs the remaining arguments.
 */
func (l *CheckDummy) Check(_ *Agent, args []string) (*CheckResult, error) {
	state := int64(0)
	output := "Dummy Check"

	if len(args) > 0 {
		res, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			res = CheckExitUnknown
			output = fmt.Sprintf("cannot parse state to int: %s", err)
		}

		state = res
	}

	if len(args) > 1 {
		output = strings.Join(args[0:], " ")
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}
