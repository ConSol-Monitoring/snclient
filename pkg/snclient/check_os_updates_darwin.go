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
	reOSXEntry   = regexp.MustCompile(`^\*\s+Label:\s+(.*)$`)
	reOSXDetails = regexp.MustCompile(`^Title:.*Version:\s(\S+), `)
)

func (l *CheckOSUpdates) addOSBackends(ctx context.Context, check *CheckData) (int, error) {
	addedCount := 0
	var err error
	err = nil

	osxAdded, osxErr := l.addOSX(ctx, check)
	if osxAdded {
		addedCount++
	}
	if osxErr != nil {
		err = fmt.Errorf("error when adding osx: %w", err)
	}

	return addedCount, err
}

// get packages from osx softwareupdate
func (l *CheckOSUpdates) addOSX(ctx context.Context, check *CheckData) (bool, error) {
	switch l.system {
	case "auto":
		if runtime.GOOS != "darwin" {
			return false, nil
		}
		_, err := os.Stat("/usr/sbin/softwareupdate")
		if os.IsNotExist(err) {
			return false, nil
		}
	case "osx":
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
	for line := range strings.SplitSeq(output, "\n") {
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
