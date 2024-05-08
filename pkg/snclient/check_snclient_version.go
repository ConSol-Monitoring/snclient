package snclient

import (
	"context"
	"runtime"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_snclient_version"] = CheckEntry{"check_snclient_version", NewCheckSNClientVersion}
	AvailableChecks["check_nscp_version"] = AvailableChecks["check_snclient_version"]
}

type CheckSNClientVersion struct{}

func NewCheckSNClientVersion() CheckHandler {
	return &CheckSNClientVersion{}
}

func (l *CheckSNClientVersion) Build() *CheckData {
	return &CheckData{
		name:        "check_snclient_version",
		description: "Check and return snclient version.",
		implemented: ALL,
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "${name} ${version} (Build: ${build}, ${go})",
		topSyntax:    "${list}",
		attributes: []CheckAttribute{
			{name: "name", description: "The name of this agent"},
			{name: "version", description: "Version string"},
			{name: "build", description: "git commit id of this build"},
		},
		exampleDefault: `
    check_snclient_version
    SNClient+ v0.12.0036 (Build: 5e351bb, go1.21.6)

There is an alias 'check_nscp_version' for this command.
	`,
	}
}

func (l *CheckSNClientVersion) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	check.listData = append(check.listData, map[string]string{
		"name":    NAME,
		"version": snc.Version(),
		"build":   Build,
		"go":      runtime.Version(),
	})

	v := convert.VersionF64(snc.Version())
	if v > 0 {
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			Name:     "version",
			Value:    v,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
	}

	return check.Finalize()
}
