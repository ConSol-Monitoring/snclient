package snclient

type CheckAlias struct {
	noCopy  noCopy
	command string
	args    []string
}

func (a *CheckAlias) Check(snc *Agent, _ []string) (*CheckResult, error) {
	return snc.runCheck(a.command, a.args), nil
}
