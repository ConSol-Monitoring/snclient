package snclient

import (
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v3/host"
)

func init() {
	AvailableChecks["check_os_version"] = CheckEntry{"check_os_version", new(CheckOSVersion)}
}

type CheckOSVersion struct{}

func (l *CheckOSVersion) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		name:        "check_os_version",
		description: "Checks the os system version.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		topSyntax: "${status}: ${platform} ${version} (arch: ${arch})",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	platform, family, version, err := host.PlatformInformation()
	if err != nil {
		return nil, fmt.Errorf("failed to get platform information: %s", err.Error())
	}

	check.details = map[string]string{
		"platform": platform,
		"family":   family,
		"version":  version,
		"arch":     runtime.GOARCH,
	}

	return check.Finalize()
}
