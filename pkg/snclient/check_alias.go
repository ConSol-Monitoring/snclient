package snclient

type CheckAlias struct {
	noCopy  noCopy
	command string
	args    []string
	config  *ConfigSection // TODO: use for check_wrap
}

func (a *CheckAlias) Check(snc *Agent, _ []string) (*CheckResult, error) {
	return snc.runCheck(a.command, a.args), nil
}
