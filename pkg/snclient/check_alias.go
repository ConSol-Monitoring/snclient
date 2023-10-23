package snclient

import "context"

type CheckAlias struct {
	noCopy  noCopy
	command string
	args    []string // arguments supplied by the alias itself
	config  *ConfigSection
}

func (a *CheckAlias) Build() *CheckData {
	return &CheckData{
		argsPassthrough: true,
	}
}

func (a *CheckAlias) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	userArgs := check.rawArgs
	var statusResult *CheckResult
	switch {
	case !checkAllowArguments(a.config, userArgs):
		statusResult = &CheckResult{
			State:  CheckExitUnknown,
			Output: "Exception processing request: Request contained arguments (check the allow arguments option).",
		}
	case !checkNastyCharacters(a.config, "", userArgs):
		statusResult = &CheckResult{
			State:  CheckExitUnknown,
			Output: "Exception processing request: Request contained illegal characters (check the allow nasty characters option).",
		}
	default:
		args := a.args
		args = append(args, userArgs...)
		statusResult = snc.runCheck(ctx, a.command, args)
	}

	statusResult.ParsePerformanceDataFromOutputCond(a.command, a.config)

	return statusResult, nil
}
