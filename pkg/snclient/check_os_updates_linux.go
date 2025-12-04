package snclient

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
)

var (
	reAPTSecurity = regexp.MustCompile(`(Debian-Security:|Ubuntu:[^/]*/[^-]*-security)`)
	reAPTEntry    = regexp.MustCompile(`^Inst\s+(\S+)\s+\[([^\[]+)\]\s+\((\S+)\s+(.*)\s+\[(\S+)\]\)`)
	reYUMEntry    = regexp.MustCompile(`^(\S+)\.(\S+)\s+(\S+)\s+(\S+)`)
)

func (l *CheckOSUpdates) addOSBackends(ctx context.Context, check *CheckData) (int, error) {
	addedCount := 0
	var err error
	err = nil

	aptAdded, aptErr := l.addAPT(ctx, check)
	if aptAdded {
		addedCount++
	}
	if aptErr != nil {
		err = fmt.Errorf("error when adding apt: %w", err)
	}

	yumAdded, yumErr := l.addYUM(ctx, check)
	if yumAdded {
		addedCount++
	}
	if yumErr != nil {
		err = fmt.Errorf("%w | error when adding yum: %w", err, yumErr)
	}

	return addedCount, err
}

// get packages from apt
func (l *CheckOSUpdates) addAPT(ctx context.Context, check *CheckData) (bool, error) {
	switch l.system {
	case "auto":
		if runtime.GOOS != "linux" {
			return false, nil
		}
		_, err := os.Stat("/usr/bin/apt-get")
		if os.IsNotExist(err) {
			return false, nil
		}
	case "apt":
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
	for line := range strings.SplitSeq(output, "\n") {
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
	switch l.system {
	case "auto":
		if runtime.GOOS != "linux" {
			return false, nil
		}
		_, err := os.Stat("/usr/bin/yum")
		if os.IsNotExist(err) {
			return false, nil
		}
	case "yum":
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
	for line := range strings.SplitSeq(output, "\n") {
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
