package snclient

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_os_updates"] = CheckEntry{"check_os_updates", NewCheckOSUpdates}
}

type CheckOSUpdates struct {
	snc             *Agent
	system          string
	update          bool
	includeOptional bool
	maxMetadataAge  string
}

func NewCheckOSUpdates() CheckHandler {
	return &CheckOSUpdates{
		update:         false,
		system:         "auto",
		maxMetadataAge: "24h",
	}
}

func (l *CheckOSUpdates) Build() *CheckData {
	return &CheckData{
		name:         "check_os_updates",
		description:  "Checks for operating system updates.",
		implemented:  Linux | Windows | Darwin,
		hasInventory: NoCallInventory,
		result:       &CheckResult{},
		args: map[string]CheckArgument{
			"-s|--system":        {value: &l.system, description: "Package system: auto, apt, yum, macos and windows (default: auto)"},
			"-u|--update":        {value: &l.update, description: "Update package list (if supported, e.g. apt-get update)"},
			"--include-optional": {value: &l.includeOptional, description: "Include optional Windows updates (default: false)"},
			"-m|--max-metadata-age": {
				value:       &l.maxMetadataAge,
				description: "If repository metadata (apt/yum/dnf) exceeds this age, return UNKNOWN. For example: 48h (default: 24h)",
			},
		},
		defaultWarning:  "count > 0",
		defaultCritical: "count_security > 0",
		detailSyntax:    "%(prefix)%(package): %(version)",
		listCombine:     "\n",
		topSyntax:       "%(status) - %(count_security) security updates / %(count) updates available.\n%(list)",
		emptyState:      CheckExitOK,
		emptySyntax:     "%(status) - no updates available",
		attributes: []CheckAttribute{
			{name: "package", description: "package name"},
			{name: "security", description: "is this a security update: 0 / 1"},
			{name: "version", description: "version string of package"},
			{name: "type", description: "Windows only: update type (1 = software, 2 = driver)"},
			{name: "is_beta", description: "Windows only: beta update (true / false)"},
			{name: "is_mandatory", description: "Windows only: mandatory update (true / false)"},
			{name: "deployment_action", description: "Windows only: deployment action (0 = none, 1 = installation, 2 = uninstallation, 3 = detection, 4 = optional installation)"},
			{name: "kb_article_ids", description: "Windows only: comma-separated KB article IDs"},
			{name: "severity", description: "Windows only: MSRC security severity"},
			{name: "is_downloaded", description: "Windows only: update downloaded (true / false, empty if unavailable)"},
			{name: "reboot_required", description: "Windows only: update requires a reboot (true / false, empty if unavailable). Does not describe the system-wide reboot state"},
			{name: "last_deployment_change_time", description: "Windows only: deployment metadata change time (UTC, ISO 8601), not when the update became pending"},
			{name: "update_id", description: "Windows only: update GUID"},
			{name: "revision_number", description: "Windows only: update revision number (empty if unavailable)"},
			{name: "browse_only", description: "Windows only: optional update signal (true / false, empty if unavailable)"},
			{name: "cve_ids", description: "Windows only: comma-separated CVE IDs"},
			{name: "max_download_size", description: "Windows only: maximum download size in bytes (empty if unavailable)"},
			{name: "driver_provider", description: "Windows only: driver provider (empty for non-driver updates or if unavailable)"},
			{name: "driver_model", description: "Windows only: driver model (empty for non-driver updates or if unavailable)"},
			{name: "driver_ver_date", description: "Windows only: driver version date (UTC, ISO 8601, empty if unavailable)"},
		},
		extraFilterAttributes: []*regexp.Regexp{
			regexp.MustCompile(`^old_version$`),
			regexp.MustCompile(`^repository$`),
			regexp.MustCompile(`^arch$`),
			regexp.MustCompile(`^prefix$`),
		},
		exampleDefault: `
    check_os_updates
    OK - no updates available |...

To receive notifications only for security updates:

    check_os_updates warn=none crit='count_security > 0'
    CRITICAL - 1 security updates / 3 updates available. |'security'=1;;0;0 'updates'=3;0;;0

On Windows, the check excludes optional updates (DeploymentAction = 4) by default.
Use **--include-optional** to include them.
Additional Windows metadata attributes are empty when unavailable or unsupported.

On DNF/APT systems, **--update** refreshes repository metadata in a private cache.
Unless you start the check as root, the SNClient service user owns this cache.

If an enabled repository is unavailable, the DNF check returns **UNKNOWN**.
Otherwise, it could report no updates for an incomplete set of repositories.

Use **--max-metadata-age** to limit the age of repository metadata, for example --max-metadata-age=24h.
If the metadata exceeds this age, the check returns **UNKNOWN**.
	`,
		exampleArgs: `warn='count > 0' crit='count_security > 0'`,
	}
}

func (l *CheckOSUpdates) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	addedOsBackendCount, osBackendAddErr := l.addOSBackends(ctx, check)

	if addedOsBackendCount == 0 {
		return nil, fmt.Errorf("no suitable package system found, supported systems are apt, yum, macos and windows. found errors: %w", osBackendAddErr)
	}
	if osBackendAddErr != nil {
		return nil, osBackendAddErr
	}

	check.listData = dedupeListData(check.listData)

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

	check.result.Metrics = append(
		check.result.Metrics,
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

// dedupeListData removes duplicate entries (same package and arch),
// preferring security updates.
func dedupeListData(listData []map[string]string) []map[string]string {
	seen := make(map[string]int, len(listData))
	written := 0
	for _, entry := range listData {
		key := entry["package"] + "\x00" + entry["arch"]
		if i, ok := seen[key]; ok {
			if entry["security"] > listData[i]["security"] {
				listData[i] = entry
			}

			continue
		}
		seen[key] = written
		listData[written] = entry
		written++
	}

	return listData[:written]
}
