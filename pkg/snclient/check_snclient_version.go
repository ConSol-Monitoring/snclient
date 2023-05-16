package snclient

import (
	"fmt"
)

func init() {
	AvailableChecks["check_snclient_version"] = CheckEntry{"check_snclient_version", new(CheckSNClientVersion)}
}

type CheckSNClientVersion struct {
	noCopy noCopy
}

/* check_snclient_version
 * Description: Returns SNClient version
 */
func (l *CheckSNClientVersion) Check(snc *Agent, _ []string) (*CheckResult, error) {
	return &CheckResult{
		State:   CheckExitOK,
		Output:  fmt.Sprintf("%s %s (Build: %s)", NAME, snc.Version(), snc.Build),
		Metrics: nil,
	}, nil
}
