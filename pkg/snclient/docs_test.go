package snclient

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocsExists(t *testing.T) {
	snc := StartTestAgent(t, "")

	for name := range AvailableChecks {
		if name == "check_index" {
			continue
		}
		if name == "check_nscp_version" {
			continue
		}
		pluginFile := fmt.Sprintf("../../docs/checks/plugins/%s.md", name)
		commandFile := fmt.Sprintf("../../docs/checks/commands/%s.md", name)
		isPlugin := false
		isCommand := false
		if _, err := os.Stat(pluginFile); err == nil {
			isPlugin = true
		}
		if _, err := os.Stat(commandFile); err == nil {
			isCommand = true
		}
		for _, osName := range []string{"windows", "linux"} {
			commandFile := fmt.Sprintf("../../docs/checks/commands/%s_%s.md", name, osName)
			if _, err := os.Stat(commandFile); err == nil {
				isCommand = true
			}
		}
		if !isCommand && !isPlugin {
			assert.Failf(t, "docs exist", "docs/checks/.../%s.md file for %s does not exist", name, name)
		}
	}

	StopTestAgent(t, snc)
}
