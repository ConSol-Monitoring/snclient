package snclient

import (
	"context"
	_ "embed"
	"fmt"
	"runtime"
	"strings"
)

//go:embed embed/scripts/windows/check_os_updates.ps1
var checkOSupdatesPS1 string

func (l *CheckOSUpdates) addOSBackends(ctx context.Context, check *CheckData) (int, error) {
	addedCount := 0
	var err error
	err = nil

	windowsAdded, windowsErr := l.addWindows(ctx, check)
	if windowsAdded {
		addedCount++
	}
	if windowsErr != nil {
		err = fmt.Errorf("error when adding windows: %w", err)
	}

	return addedCount, err
}

// get packages from windows powershell
func (l *CheckOSUpdates) addWindows(ctx context.Context, check *CheckData) (bool, error) {
	switch l.system {
	case "auto":
		if runtime.GOOS != "windows" {
			return false, nil
		}
	case "windows":
	default:
		return false, nil
	}

	// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nf-wuapi-iupdatesearcher-search
	// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdate
	cmd := powerShellCmd(ctx, checkOSupdatesPS1)
	if l.update {
		cmd.Env = append(cmd.Env, "ONLINE_SEARCH=1")
	}
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
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "Category: ") {
			if strings.Contains(line, "Security") || strings.Contains(line, "Critical") {
				lastEntry["security"] = "1"
			}

			continue
		}
		if pkg, ok := strings.CutPrefix(line, "Title: "); ok {
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
