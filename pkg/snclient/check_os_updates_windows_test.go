package snclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWindowsUpdates(t *testing.T) {
	check := &CheckData{}
	handler := &CheckOSUpdates{}
	err := handler.parseWindows(`[
		{"Title":"Security update\nwith details", "Categories":["Security Updates"], "Type":1, "IsBeta":false, "IsMandatory":true, "DeploymentAction":1},
		{"Title":"Driver", "Categories":[], "Type":2, "IsBeta":true, "IsMandatory":false, "DeploymentAction":4},
		{"Title":"Critical update", "Categories":["Critical Updates"], "Type":1, "IsBeta":false, "IsMandatory":false, "DeploymentAction":1}
	]`, check)
	require.NoError(t, err)
	require.Len(t, check.listData, 3)
	assert.Equal(t, "Security update\nwith details", check.listData[0]["package"])
	assert.Equal(t, "1", check.listData[0]["security"])
	assert.Equal(t, "1", check.listData[0]["type"])
	assert.Equal(t, "false", check.listData[0]["is_beta"])
	assert.Equal(t, "true", check.listData[0]["is_mandatory"])
	assert.Equal(t, "1", check.listData[0]["deployment_action"])
	assert.Equal(t, "0", check.listData[1]["security"])
	assert.Equal(t, "2", check.listData[1]["type"])
	assert.Equal(t, "true", check.listData[1]["is_beta"])
	assert.Equal(t, "false", check.listData[1]["is_mandatory"])
	assert.Equal(t, "4", check.listData[1]["deployment_action"])
	assert.Equal(t, "1", check.listData[2]["security"])

	check.listData = nil
	require.NoError(t, handler.parseWindows(`[]`, check))
	assert.Empty(t, check.listData)
	require.NoError(t, handler.parseWindows(`[{"Title":"Single update","Categories":[]}]`, check))
	require.Len(t, check.listData, 1)

	for _, output := range []string{"", "not JSON", `{}`, `[{"Type":"driver"}]`} {
		require.Error(t, handler.parseWindows(output, check))
		require.Len(t, check.listData, 1)
	}
}

func TestWindowsUpdatesMissingProperties(t *testing.T) {
	const mockSession = `
function New-Object {
    param($ComObject)
    $entry = [PSCustomObject]@{
        Title = 'Older interface'; Categories = @(); Type = 1
        IsBeta = $false; IsMandatory = $false; DeploymentAction = 1
        IsDownloaded = $false; LastDeploymentChangeTime = [datetime]'2026-11-04T00:00:00Z'
        MaxDownloadSize = [decimal]30536537
    }
    $entry | Add-Member ScriptProperty CveIDs { throw 'Unsupported property' }
    $entry | Add-Member ScriptProperty DriverModel { throw 'Unsupported property' }
    $searcher = [PSCustomObject]@{ Online = 0; Result = [PSCustomObject]@{ Updates = @($entry) } }
    $searcher | Add-Member ScriptMethod Search { param($criteria); $this.Result }
    $session = [PSCustomObject]@{ Searcher = $searcher }
    $session | Add-Member ScriptMethod CreateUpdateSearcher { $this.Searcher }
    return $session
}
`
	cmd, err := powerShellCmd(context.Background(), mockSession+checkOSupdatesPS1)
	require.NoError(t, err)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	check := &CheckData{}
	handler := &CheckOSUpdates{}
	require.NoError(t, handler.parseWindows(string(output), check))
	require.Len(t, check.listData, 1)
	assert.Equal(t, "Older interface", check.listData[0]["package"])
	assert.Equal(t, "false", check.listData[0]["is_downloaded"])
	assert.Equal(t, "30536537", check.listData[0]["max_download_size"])
	assert.Equal(t, "2026-11-04T00:00:00.0000000Z", check.listData[0]["last_deployment_change_time"])
	assert.Empty(t, check.listData[0]["cve_ids"])
	assert.Empty(t, check.listData[0]["severity"])
	assert.Empty(t, check.listData[0]["reboot_required"])
	assert.Empty(t, check.listData[0]["driver_provider"])
	assert.Empty(t, check.listData[0]["driver_model"])
}

func TestParseWindowsUpdateOptionalProperties(t *testing.T) {
	check := &CheckData{}
	handler := &CheckOSUpdates{}
	require.NoError(t, handler.parseWindows(`[{
		"Title":"Update", "KBArticleIDs":["1234567","7654321"], "MsrcSeverity":"Critical",
		"IsDownloaded":false, "RebootRequired":true, "BrowseOnly":false,
		"LastDeploymentChangeTime":"2026-11-04T00:00:00Z", "UpdateID":"update-guid", "RevisionNumber":0,
		"CveIDs":["CVE-2026-1234","CVE-2026-5678"], "MaxDownloadSize":30536537,
		"DriverProvider":"Microsoft", "DriverModel":"Voice Clarity", "DriverVerDate":"2026-10-16T00:00:00Z"
	}]`, check))
	require.Len(t, check.listData, 1)
	expected := map[string]string{
		"kb_article_ids":              "1234567, 7654321",
		"severity":                    "Critical",
		"is_downloaded":               "false",
		"reboot_required":             "true",
		"browse_only":                 "false",
		"last_deployment_change_time": "2026-11-04T00:00:00Z",
		"update_id":                   "update-guid",
		"revision_number":             "0",
		"cve_ids":                     "CVE-2026-1234, CVE-2026-5678",
		"max_download_size":           "30536537",
		"driver_provider":             "Microsoft",
		"driver_model":                "Voice Clarity",
		"driver_ver_date":             "2026-10-16T00:00:00Z",
	}
	for name, value := range expected {
		assert.Equal(t, value, check.listData[0][name], name)
	}

	for _, output := range []string{
		`[{"Title":"Old interface"}]`,
		`[{"Title":"Unavailable properties", "KBArticleIDs":null, "MsrcSeverity":null,
		"IsDownloaded":null, "RebootRequired":null, "BrowseOnly":null,
		"LastDeploymentChangeTime":null, "UpdateID":null, "RevisionNumber":null,
		"CveIDs":null, "MaxDownloadSize":null, "DriverProvider":null, "DriverModel":null, "DriverVerDate":null}]`,
	} {
		check.listData = nil
		require.NoError(t, handler.parseWindows(output, check))
		require.Len(t, check.listData, 1)
		for name := range expected {
			value, exists := check.listData[0][name]
			assert.True(t, exists, name)
			assert.Empty(t, value, name)
		}
	}
}
