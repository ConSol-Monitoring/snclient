package snclient

type CheckAlias struct {
	noCopy  noCopy
	command string
	args    []string // arguments supplied by the alias itself
	config  *ConfigSection
}

func (a *CheckAlias) Check(snc *Agent, userArgs []string) (*CheckResult, error) {
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
		statusResult = snc.runCheck(a.command, args)
	}

	statusResult.ParsePerformanceDataFromOutputCond(a.command, a.config)

	return statusResult, nil
}
