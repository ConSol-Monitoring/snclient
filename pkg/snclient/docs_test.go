package snclient

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocsExists(t *testing.T) {
	snc := StartTestAgent(t, "")

	skipChecks := []string{"check_index", "check_nscp_version", "CheckCounter"}

	assert.GreaterOrEqualf(t, len(AvailableChecks), 25, "there should be checks available")

	for name := range AvailableChecks {
		if slices.Contains(skipChecks, name) {
			continue
		}
		checkType := fmt.Sprintf("%T", AvailableChecks[name].Handler())
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
			t.Logf("%s: %s", name, checkType)
			assert.Failf(t, "docs exist", "docs/checks/.../%s.md file for %s does not exist", name, name)
		}
	}

	StopTestAgent(t, snc)
}
