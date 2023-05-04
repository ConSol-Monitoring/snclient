package snclient

import (
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_os_version"] = CheckEntry{"check_os_version", new(CheckOSVersion)}
}

type CheckOSVersion struct {
	noCopy noCopy
	data   CheckData
}

/* check_os_version
 * Description: Checks the os version
 */
func (l *CheckOSVersion) Check(args []string) (*CheckResult, error) {
	state := CheckExitOK
	_, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	metrics := []*CheckMetric{}

	platform, _, version, err := host.PlatformInformation()
	if err != nil {
		return nil, fmt.Errorf("failed to get platform information: %s", err.Error())
	}

	variables := map[string]string{
		"version": fmt.Sprintf("%s %s (arch:%s)", platform, version, runtime.GOARCH),
	}

	switch {
	case CompareMetrics(variables, l.data.critThreshold):
		state = CheckExitCritical
	case CompareMetrics(variables, l.data.warnThreshold):
		state = CheckExitWarning
	}

	return &CheckResult{
		State:   state,
		Output:  fmt.Sprintf("${status} - %s", variables["version"]),
		Metrics: metrics,
	}, nil
}
