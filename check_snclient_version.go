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
func (l *CheckSNClientVersion) Check(_ []string) (*CheckResult, error) {
	return &CheckResult{
		State:   CheckExitOK,
		Output:  fmt.Sprintf("%s v%s.%s", NAME, VERSION, agent.Revision),
		Metrics: nil,
	}, nil
}
