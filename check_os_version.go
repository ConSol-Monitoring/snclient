package snclient

import (
	"fmt"
	"runtime"
	"strconv"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_os_version"] = CheckEntry{"check_os_version", new(CheckOSVersion)}
}

type CheckOSVersion struct {
	noCopy noCopy
}

/* check_os_version
 * Description: Checks the os version
 */
func (l *CheckOSVersion) Check(_ []string) (*CheckResult, error) {
	metrics := []*CheckMetric{}

	platform, _, version, err := host.PlatformInformation()
	if err != nil {
		return nil, fmt.Errorf("failed to get platform information: %s", err.Error())
	}

	versionFloat, err := strconv.ParseFloat(version, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse platform version to float: %s", err.Error())
	}

	metrics = append(metrics, &CheckMetric{
		Name:  "version",
		Value: versionFloat,
	})

	return &CheckResult{
		State:   0,
		Output:  fmt.Sprintf("OK - %s %s (arch:%s)", platform, version, runtime.GOARCH),
		Metrics: metrics,
	}, nil
}
