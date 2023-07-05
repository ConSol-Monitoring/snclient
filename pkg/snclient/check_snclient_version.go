package snclient

func init() {
	AvailableChecks["check_snclient_version"] = CheckEntry{"check_snclient_version", new(CheckSNClientVersion)}
}

type CheckSNClientVersion struct {
	noCopy noCopy
}

func (l *CheckSNClientVersion) Check(snc *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		name:        "check_snclient_version",
		description: "Check and return snclient version.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "${name} ${version} (Build: ${build})",
		topSyntax:    "${list}",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	check.listData = append(check.listData, map[string]string{
		"name":    NAME,
		"version": snc.Version(),
		"build":   Build,
	})

	return check.Finalize()
}
