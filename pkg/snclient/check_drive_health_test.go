package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var smartctlScanOpenOutput = `{
  "json_format_version": [
    1,
    0
  ],
  "smartctl": {
    "version": [
      7,
      4
    ],
    "pre_release": false,
    "svn_revision": "5530",
    "platform_info": "x86_64-linux-6.12.48+deb13-amd64",
    "build_info": "(local build)",
    "argv": [
      "smartctl",
      "--scan-open",
      "--json"
    ],
    "exit_status": 0
  },
  "devices": [
    {
      "name": "/dev/nvme0",
      "info_name": "/dev/nvme0",
      "type": "nvme",
      "protocol": "NVMe"
    }
  ]
}`

func TestParsingSmartctlOpen(t *testing.T) {
	var output SmartctlJSONOutputScanOpen
	err := json.Unmarshal([]byte(smartctlScanOpenOutput), &output)
	assert.NoError(t, err)
}

func TestSmartctlScanOpen(t *testing.T) {
	_, err := SmartctlScanOpen()
	assert.NoError(t, err)
}

var smartctlStartTestOutput = `{
  "json_format_version": [
    1,
    0
  ],
  "smartctl": {
    "version": [
      7,
      4
    ],
    "pre_release": false,
    "svn_revision": "5530",
    "platform_info": "x86_64-linux-6.12.48+deb13-amd64",
    "build_info": "(local build)",
    "argv": [
      "smartctl",
      "--json",
      "--test",
      "short",
      "/dev/nvme0"
    ],
    "exit_status": 0
  },
  "local_time": {
    "time_t": 1762790307,
    "asctime": "Mon Nov 10 16:58:27 2025 CET"
  },
  "device": {
    "name": "/dev/nvme0",
    "info_name": "/dev/nvme0",
    "type": "nvme",
    "protocol": "NVMe"
  }
}`

func TestParsingSmartctlStartTest(t *testing.T) {
	var output SmartctlJSONOutputStartTest
	err := json.Unmarshal([]byte(smartctlStartTestOutput), &output)
	assert.NoError(t, err)
}

func TestParsingSmartctlXall(t *testing.T) {
	smartctlJSONFilePaths, fileDiscoveryError := filepath.Glob("t/smartctl_outputs/*.json")
	require.NoError(t, fileDiscoveryError)

	for _, smartctlJSONFilePath := range smartctlJSONFilePaths {
		t.Run(smartctlJSONFilePath, func(t *testing.T) {
			fileContent, err := os.ReadFile(smartctlJSONFilePath)
			require.NoError(t, err)

			if !json.Valid(fileContent) {
				// handle the error here
				assert.Fail(t, "Json is not valid")
			}

			var output SmartctlJSONOutputXall
			err = json.Unmarshal(fileContent, &output)
			assert.NoError(t, err)
		})
	}
}

// No need to do local tests
// func TestLocal(t *testing.T) {
// 	snc := StartTestAgent(t, "")

// 	res := snc.RunCheck("check_drive_health", []string{""})
// 	assert.Equalf(t, CheckExitOK, res.State, "state ok")

// 	StopTestAgent(t, snc)
// }
