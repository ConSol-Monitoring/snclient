package snclient

import (
	"fmt"
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
func (l *CheckOSVersion) Check(args []string) (*CheckResult, error) {
	metrics := []*CheckMetric{}

	platform, _, version, err := host.PlatformInformation()
	if err != nil {
		return &CheckResult{
			State:   ExitCodeUnknown,
			Output:  err.Error(),
			Metrics: metrics,
		}, nil
	}

	v, err := strconv.ParseFloat(version, 64)
	metrics = append(metrics, &CheckMetric{
		Name:  "version",
		Value: v,
	})

	return &CheckResult{
		State:   0,
		Output:  fmt.Sprintf("OK - %s %s", platform, version),
		Metrics: metrics,
	}, nil
}
