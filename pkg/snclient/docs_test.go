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
		pluginFile := fmt.Sprintf("docs/checks/plugins/%s.md", name)
		commandFile := fmt.Sprintf("docs/checks/commands/%s.md", name)
		isPlugin := false
		isCommand := false
		if _, err := os.Stat(pluginFile); os.IsNotExist(err) {
			isPlugin = true
		}
		if _, err := os.Stat(commandFile); os.IsNotExist(err) {
			isCommand = true
		}
		if !isCommand && !isPlugin {
			assert.Failf(t, "docs exist", "docs/checks/... file for %s does not exist", name)
		}
	}

	StopTestAgent(t, snc)
}
