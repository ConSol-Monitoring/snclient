package snclient

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_os_updates"] = CheckEntry{"check_os_updates", NewCheckOSUpdates}
}

type CheckOSUpdates struct {
	snc    *Agent
	system string
	update bool
}

func NewCheckOSUpdates() CheckHandler {
	return &CheckOSUpdates{
		update: false,
		system: "auto",
	}
}

func (l *CheckOSUpdates) Build() *CheckData {
	return &CheckData{
		name:         "check_os_updates",
		description:  "Checks for OS system updates.",
		implemented:  Linux | Windows | Darwin,
		hasInventory: NoCallInventory,
		result:       &CheckResult{},
		args: map[string]CheckArgument{
			"-s|--system": {value: &l.system, description: "Package system: auto, apt, yum, osx and windows (default: auto)"},
			"-u|--update": {value: &l.update, description: "Update package list (if supported, ex.: apt-get update)"},
		},
		defaultWarning:  "count > 0",
		defaultCritical: "count_security > 0",
		detailSyntax:    "${prefix}${package}: ${version}",
		listCombine:     "\n",
		topSyntax:       "%(status) - %{count_security} security updates / %{count} updates available.\n%{list}",
		emptyState:      CheckExitOK,
		emptySyntax:     "%(status) - no updates available",
		attributes: []CheckAttribute{
			{name: "package", description: "package name"},
			{name: "security", description: "is this a security update: 0 / 1"},
			{name: "version", description: "version string of package"},
		},
		exampleDefault: `
    check_os_updates
    OK - no updates available |...

If you only want to be notified about security related updates:

    check_os_updates warn=none crit='count_security > 0'
    CRITICAL - 1 security updates / 3 updates available. |'security'=1;;0;0 'updates'=3;0;;0
	`,
		exampleArgs: `warn='count > 0' crit='count_security > 0'`,
	}
}

func (l *CheckOSUpdates) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	addedOsBackendCount, osBackendAddErr := l.addOSBackends(ctx, check)

	if addedOsBackendCount == 0 {
		return nil, fmt.Errorf("no suitable package system found, supported systems are apt, yum, osx and windows. found errors: %w", osBackendAddErr)
	}

	count := 0
	countSecurity := 0
	for _, entry := range check.listData {
		entry["prefix"] = ""
		if entry["security"] == "1" {
			countSecurity++
			entry["prefix"] = "[SECURITY] "
		} else {
			count++
		}
	}

	// sort updates by security status and package name
	slices.SortFunc(check.listData, func(a, b map[string]string) int {
		switch cmp.Compare(b["security"], a["security"]) {
		case -1:
			return -1
		case 1:
			return 1
		default:
			return cmp.Compare(a["package"], b["package"])
		}
	})

	// apply filter
	check.listData = check.Filter(check.filter, check.listData)

	check.details = map[string]string{
		"count":          fmt.Sprintf("%d", count),
		"count_security": fmt.Sprintf("%d", countSecurity),
	}

	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			ThresholdName: "count_security",
			Name:          "security",
			Unit:          "",
			Value:         convert.Int64(check.details["count_security"]),
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
			Min:           &Zero,
		},
		&CheckMetric{
			ThresholdName: "count",
			Name:          "updates",
			Unit:          "",
			Value:         convert.Int64(check.details["count"]),
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
			Min:           &Zero,
		},
	)

	return check.Finalize()
}
