package snclient

import "context"

func init() {
	AvailableChecks["check_snclient_version"] = CheckEntry{"check_snclient_version", new(CheckSNClientVersion)}
	AvailableChecks["check_nscp_version"] = AvailableChecks["check_snclient_version"]
}

type CheckSNClientVersion struct{}

func (l *CheckSNClientVersion) Build() *CheckData {
	return &CheckData{
		name:        "check_snclient_version",
		description: "Check and return snclient version.",
		implemented: ALL,
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "${name} ${version} (Build: ${build})",
		topSyntax:    "${list}",
		attributes: []CheckAttribute{
			{name: "name", description: "The name of this agent"},
			{name: "version", description: "Version string"},
			{name: "build", description: "git commit id of this build"},
		},
		exampleDefault: `
    check_snclient_version
    SNClient+ v0.12.0036 (Build: 5e351bb)

There is an alias 'check_nscp_version' for this command.
	`,
	}
}

func (l *CheckSNClientVersion) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	check.listData = append(check.listData, map[string]string{
		"name":    NAME,
		"version": snc.Version(),
		"build":   Build,
	})

	return check.Finalize()
}
