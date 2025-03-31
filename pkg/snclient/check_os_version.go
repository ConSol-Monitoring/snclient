package snclient

import (
	"context"
	"fmt"
	"runtime"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/shirou/gopsutil/v4/host"
)

func init() {
	AvailableChecks["check_os_version"] = CheckEntry{"check_os_version", NewCheckOSVersion}
}

type CheckOSVersion struct{}

func NewCheckOSVersion() CheckHandler {
	return &CheckOSVersion{}
}

func (l *CheckOSVersion) Build() *CheckData {
	return &CheckData{
		name:         "check_os_version",
		description:  "Checks the os system version.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		topSyntax:    "%(status) - ${list}",
		detailSyntax: "${platform} ${version} (arch: ${arch})",
		attributes: []CheckAttribute{
			{name: "platform", description: "Platform of the OS"},
			{name: "family", description: "OS Family"},
			{name: "version", description: "Full version number"},
			{name: "arch", description: "OS architecture (go arch, ex.: amd64)"},
			{name: "os", description: "OS name"},
			{name: "hostname", description: "hostname (probably without fqdn)"},
			{name: "kernel_arch", description: "kernel architecture, ex.: x86_64"},
			{name: "kernel_version", description: "kernel version"},
		},
		exampleDefault: `
    check_os_version
    OK - Microsoft Windows 10 Pro 10.0.19045.2728 Build 19045.2728 (arch: amd64)
	`,
	}
}

func (l *CheckOSVersion) Check(ctx context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	platform, family, version, err := host.PlatformInformationWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get platform information: %s", err.Error())
	}

	if runtime.GOOS == "windows" {
		version, err = host.KernelVersionWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get build information: %s", err.Error())
		}
	}

	hostInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host information: %s", err.Error())
	}

	check.listData = []map[string]string{{
		"platform":       platform,
		"family":         family,
		"version":        version,
		"arch":           runtime.GOARCH,
		"os":             runtime.GOOS,
		"hostname":       hostInfo.Hostname,
		"kernel_version": hostInfo.KernelVersion,
		"kernel_arch":    hostInfo.KernelArch,
	}}

	v := convert.VersionF64(version)
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
