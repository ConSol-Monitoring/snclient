package snclient

import (
	"context"
	_ "embed"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
)

//go:embed embed/scripts/windows/check_os_updates.ps1
var checkOSupdatesPS1 string

func (l *CheckOSUpdates) addOSBackends(ctx context.Context, check *CheckData) (addedCount int, err error) {
	windowsAdded, windowsErr := l.addWindows(ctx, check)
	if windowsAdded {
		addedCount++
	}
	if windowsErr != nil {
		err = fmt.Errorf("error when adding windows: %s", windowsErr.Error())
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
	cmd, err := powerShellCmd(ctx, checkOSupdatesPS1)
	if err != nil {
		return false, fmt.Errorf("error when building a powershell command: %s", err.Error())
	}
	if l.update {
		cmd.Env = append(cmd.Env, "ONLINE_SEARCH=1")
	}
	if l.includeOptional {
		cmd.Env = append(cmd.Env, "INCLUDE_OPTIONAL=1")
	}
	output, stderr, exitCode, _, err := l.snc.runExternalCommand(ctx, cmd, l.snc.getBuiltinCmdTimeout())
	if err != nil {
		return true, fmt.Errorf("getting pending updates failed: %s\n%s", err.Error(), stderr)
	}
	if exitCode != 0 {
		return true, fmt.Errorf("getting pending updates failed: %s\n%s", output, stderr)
	}

	return true, l.parseWindows(output, check)
}

func (l *CheckOSUpdates) parseWindows(output string, check *CheckData) error {
	var updates []struct {
		Title                    string
		Categories               []string
		Type                     int
		IsBeta                   bool
		IsMandatory              bool
		DeploymentAction         int
		KBArticleIDs             []string
		MsrcSeverity             string
		IsDownloaded             *bool
		RebootRequired           *bool
		LastDeploymentChangeTime string
		UpdateID                 string
		RevisionNumber           *int64
		BrowseOnly               *bool
		CveIDs                   []string
		MaxDownloadSize          *int64
		DriverProvider           string
		DriverModel              string
		DriverVerDate            string
	}
	if err := json.Unmarshal([]byte(output), &updates); err != nil {
		return fmt.Errorf("could not unmarshal Windows updates: %w", err)
	}

	for index := range updates {
		update := &updates[index]
		entry := map[string]string{
			"security":                    "0",
			"package":                     update.Title,
			"version":                     "",
			"old_version":                 "",
			"repository":                  "",
			"arch":                        "",
			"type":                        strconv.Itoa(update.Type),
			"is_beta":                     strconv.FormatBool(update.IsBeta),
			"is_mandatory":                strconv.FormatBool(update.IsMandatory),
			"deployment_action":           strconv.Itoa(update.DeploymentAction),
			"kb_article_ids":              strings.Join(update.KBArticleIDs, ", "),
			"severity":                    update.MsrcSeverity,
			"last_deployment_change_time": update.LastDeploymentChangeTime,
			"update_id":                   update.UpdateID,
			"cve_ids":                     strings.Join(update.CveIDs, ", "),
			"driver_provider":             update.DriverProvider,
			"driver_model":                update.DriverModel,
			"driver_ver_date":             update.DriverVerDate,
		}
		for name, value := range map[string]*bool{
			"is_downloaded":   update.IsDownloaded,
			"reboot_required": update.RebootRequired,
			"browse_only":     update.BrowseOnly,
		} {
			entry[name] = ""
			if value != nil {
				entry[name] = strconv.FormatBool(*value)
			}
		}
		for name, value := range map[string]*int64{
			"revision_number":   update.RevisionNumber,
			"max_download_size": update.MaxDownloadSize,
		} {
			entry[name] = ""
			if value != nil {
				entry[name] = strconv.FormatInt(*value, 10)
			}
		}
		for _, category := range update.Categories {
			if strings.Contains(category, "Security") || strings.Contains(category, "Critical") {
				entry["security"] = "1"
			}
		}
		check.listData = append(check.listData, entry)
	}

	return nil
}
