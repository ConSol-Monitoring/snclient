package snclient

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"pkg/convert"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_os_updates"] = CheckEntry{"check_os_updates", NewCheckOSUpdates}
}

var (
	reAPTSecurity = regexp.MustCompile(`(Debian-Security:|Ubuntu:[^/]*/[^-]*-security)`)
	reAPTEntry    = regexp.MustCompile(`^Inst\s+(\S+)\s+\[([^\[]+)\]\s+\((\S+)\s+(.*)\s+\[(\S+)\]\)`)
	reYUMEntry    = regexp.MustCompile(`^(\S+)\.(\S+)\s+(\S+)\s+(\S+)`)
	reOSXEntry    = regexp.MustCompile(`^\*\s+Label:\s+(.*)$`)
	reOSXDetails  = regexp.MustCompile(`^Title:.*Version:\s(\S+), `)
)

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

	found := 0
	ok, err := l.addAPT(ctx, check)
	if err != nil {
		return nil, err
	}
	if ok {
		found++
	}

	ok, err = l.addYUM(ctx, check)
	if err != nil {
		return nil, err
	}
	if ok {
		found++
	}

	ok, err = l.addOSX(ctx, check)
	if err != nil {
		return nil, err
	}
	if ok {
		found++
	}

	ok, err = l.addWindows(ctx, check)
	if err != nil {
		return nil, err
	}
	if ok {
		found++
	}

	if found == 0 {
		return nil, fmt.Errorf("no suitable package system found, supported systems are apt, yum, osx and windows")
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

// get packages from apt
func (l *CheckOSUpdates) addAPT(ctx context.Context, check *CheckData) (bool, error) {
	switch {
	case l.system == "auto":
		if runtime.GOOS != "linux" {
			return false, nil
		}
		_, err := os.Stat("/usr/bin/apt-get")
		if os.IsNotExist(err) {
			return false, nil
		}
	case l.system == "apt":
	default:
		return false, nil
	}

	if l.update {
		output, stderr, rc, err := l.snc.execCommand(ctx, "apt-get update", DefaultCmdTimeout)
		if err != nil {
			return true, fmt.Errorf("apt-get update failed: %s\n%s", err.Error(), stderr)
		}
		if rc != 0 {
			return true, fmt.Errorf("apt-get update failed: %s\n%s", output, stderr)
		}
	}

	output, stderr, rc, err := l.snc.execCommand(ctx, "apt-get upgrade -o 'Debug::NoLocking=true' -s -qq", DefaultCmdTimeout)
	if err != nil {
		return true, fmt.Errorf("apt-get upgrade failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return true, fmt.Errorf("apt-get upgrade failed: %s\n%s", output, stderr)
	}

	l.parseAPT(output, check)

	return true, nil
}

func (l *CheckOSUpdates) parseAPT(output string, check *CheckData) {
	for _, line := range strings.Split(output, "\n") {
		matches := reAPTEntry.FindStringSubmatch(line)
		security := "0"
		if reAPTSecurity.MatchString(line) {
			security = "1"
		}
		if len(matches) < 5 {
			continue
		}
		check.listData = append(check.listData, map[string]string{
			"security":    security,
			"package":     matches[1],
			"version":     matches[3],
			"old_version": matches[2],
			"repository":  matches[4],
			"arch":        matches[5],
		})
	}
}

// get packages from yum
func (l *CheckOSUpdates) addYUM(ctx context.Context, check *CheckData) (bool, error) {
	switch {
	case l.system == "auto":
		if runtime.GOOS != "linux" {
			return false, nil
		}
		_, err := os.Stat("/usr/bin/yum")
		if os.IsNotExist(err) {
			return false, nil
		}
	case l.system == "yum":
	default:
		return false, nil
	}

	yumOpts := " -C"
	if l.update {
		yumOpts = ""
	}

	output, stderr, exitCode, err := l.snc.execCommand(ctx, "yum check-update --security -q"+yumOpts, DefaultCmdTimeout)
	if err != nil {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 && exitCode != 100 {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", output, stderr)
	}
	packageLookup := l.parseYUM(output, "1", check, nil)

	output, stderr, exitCode, err = l.snc.execCommand(ctx, "yum check-update -q"+yumOpts, DefaultCmdTimeout)
	if err != nil {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 && exitCode != 100 {
		return true, fmt.Errorf("yum check-update failed: %s\n%s", output, stderr)
	}
	l.parseYUM(output, "0", check, packageLookup)

	return true, nil
}

func (l *CheckOSUpdates) parseYUM(output, security string, check *CheckData, skipPackages map[string]bool) map[string]bool {
	packages := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Obsoleting Packages") {
			break
		}
		matches := reYUMEntry.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		if skipPackages[matches[1]] {
			continue
		}
		packages[matches[1]] = true
		check.listData = append(check.listData, map[string]string{
			"security":    security,
			"package":     matches[1],
			"version":     matches[2],
			"old_version": "",
			"repository":  matches[3],
			"arch":        matches[2],
		})
	}

	return packages
}

// get packages from osx softwareupdate
func (l *CheckOSUpdates) addOSX(ctx context.Context, check *CheckData) (bool, error) {
	switch {
	case l.system == "auto":
		if runtime.GOOS != "darwin" {
			return false, nil
		}
		_, err := os.Stat("/usr/sbin/softwareupdate")
		if os.IsNotExist(err) {
			return false, nil
		}
	case l.system == "osx":
	default:
		return false, nil
	}

	opts := " --no-scan"
	if l.update {
		opts = ""
	}

	output, stderr, exitCode, err := l.snc.execCommand(ctx, "softwareupdate -l"+opts, DefaultCmdTimeout)
	if err != nil {
		return true, fmt.Errorf("softwareupdate failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 {
		return true, fmt.Errorf("softwareupdate failed: %s\n%s", output, stderr)
	}

	l.parseOSX(output, check)

	return true, nil
}

func (l *CheckOSUpdates) parseOSX(output string, check *CheckData) {
	var lastEntry map[string]string
	for _, line := range strings.Split(output, "\n") {
		if lastEntry != nil {
			matches := reOSXDetails.FindStringSubmatch(line)
			if len(matches) > 1 {
				lastEntry["version"] = matches[1]

				continue
			}
		}
		matches := reOSXEntry.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		entry := map[string]string{
			"security":    "0",
			"package":     matches[1],
			"version":     "",
			"old_version": "",
			"repository":  "",
			"arch":        "",
		}
		check.listData = append(check.listData, entry)
		lastEntry = entry
	}
}

// get packages from windows powershell
func (l *CheckOSUpdates) addWindows(ctx context.Context, check *CheckData) (bool, error) {
	switch {
	case l.system == "auto":
		if runtime.GOOS != "windows" {
			return false, nil
		}
	case l.system == "windows":
	default:
		return false, nil
	}

	online := "0"
	if l.update {
		online = "1"
	}

	// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nf-wuapi-iupdatesearcher-search
	// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdate
	updates := `
		$update = new-object -com Microsoft.update.Session
		$searcher = $update.CreateUpdateSearcher()
		$searcher.Online = ` + online + `
		$pending = $searcher.Search('IsInstalled=0 AND IsHidden=0')
		foreach($entry in $pending.Updates) {
			Write-host Title: $entry.Title
			foreach($cat in $entry.Categories) {
				Write-host Category: $cat.Name
			}
		}

	`
	cmd := powerShellCmd(ctx, updates)
	output, stderr, exitCode, _, err := l.snc.runExternalCommand(ctx, cmd, DefaultCmdTimeout)
	if err != nil {
		return true, fmt.Errorf("getting pending updates failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 {
		return true, fmt.Errorf("getting pending updates failed: %s\n%s", output, stderr)
	}

	l.parseWindows(output, check)

	return true, nil
}

func (l *CheckOSUpdates) parseWindows(output string, check *CheckData) {
	var lastEntry map[string]string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Category: ") {
			if strings.Contains(line, "Security") || strings.Contains(line, "Critical") {
				lastEntry["security"] = "1"
			}

			continue
		}
		if strings.HasPrefix(line, "Title: ") {
			pkg := strings.TrimPrefix(line, "Title: ")
			entry := map[string]string{
				"security":    "0",
				"package":     pkg,
				"version":     "",
				"old_version": "",
				"repository":  "",
				"arch":        "",
			}
			check.listData = append(check.listData, entry)
			lastEntry = entry
		}
	}
}
