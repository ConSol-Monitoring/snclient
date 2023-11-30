package snclient

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
)

func TestDocsExists(t *testing.T) {
	snc := StartTestAgent(t, "")

	skipChecks := []string{"check_index", "check_nscp_version"}
	skipTypes := []string{"*snclient.CheckAlias", "*snclient.CheckWrap"}

	for name := range AvailableChecks {
		if slices.Contains(skipChecks, name) {
			continue
		}
		checkType := fmt.Sprintf("%T", AvailableChecks[name].Handler())
		if slices.Contains(skipTypes, checkType) {
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
			t.Logf("%s: %s", name, checkType)
			assert.Failf(t, "docs exist", "docs/checks/.../%s.md file for %s does not exist", name, name)
		}
	}

	StopTestAgent(t, snc)
}
