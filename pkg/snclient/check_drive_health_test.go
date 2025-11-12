package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

var smartctl_scan_open_output string = `{
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
	var output SmartctlJsonOutputScanOpen
	err := json.Unmarshal([]byte(smartctl_scan_open_output), &output)
	assert.NoError(t, err)
}

func TestSmartctlScanOpen(t *testing.T) {

	_, err := SmartctlScanOpen()
	assert.NoError(t, err)

}

var smartctl_start_test_output string = `{
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
	var output SmartctlJsonOutputStartTest
	err := json.Unmarshal([]byte(smartctl_start_test_output), &output)
	assert.NoError(t, err)
}

func TestParsingSmartctlXall(t *testing.T) {

	json_file_paths, err := filepath.Glob("t/smartctl_outputs/*.json")
	assert.NoError(t, err)

	for _, json_file_path := range json_file_paths {

		t.Run(json_file_path, func(t *testing.T) {

			file_content, err := os.ReadFile(json_file_path)
			assert.NoError(t, err)

			if !json.Valid(file_content) {
				// handle the error here
				assert.Fail(t, "Json is not valid")
			}

			var output SmartctlJsonOutputXall
			err = json.Unmarshal(file_content, &output)
			assert.NoError(t, err)
		})
	}
}

func TestNvme(t *testing.T) {
	snc := StartTestAgent(t, "")

	res := snc.RunCheck("check_drive_health", []string{"test_type='short'", "drive_filter='/dev/nvme0'"})
	assert.Equalf(t, CheckExitOK, res.State, "state ok")

	StopTestAgent(t, snc)
}
