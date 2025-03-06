package commands

import (
	"os"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/spf13/cobra"
)

func init() {
	configCmd := &cobra.Command{
		Use:     "config [cmd]",
		Aliases: []string{"conf"},
		Short:   "Run configuration test or dump config in use.",
		Long:    `Config test verifies the current configuration files.`,
		Example: `  * Run configuration check

%> snclient config check
`,
	}
	rootCmd.AddCommand(configCmd)

	// config check
	configCmd.AddCommand(&cobra.Command{
		Use:     "check",
		Aliases: []string{"test"},
		Short:   "Checks current configuration files.",
		Run:     configTest,
	})
}

func configTest(_ *cobra.Command, _ []string) {
	agentFlags.Mode = snclient.ModeOneShot
	setInteractiveStdoutLogger()
	snc := snclient.NewAgentSimple(agentFlags)
	files, defaultLocations := snc.FindConfigFiles()

	if len(files) == 0 {
		snc.Log.Errorf("no config file supplied (--config=..) and no readable config file found in default locations (%s)",
			strings.Join(defaultLocations, ", "))
		os.Exit(snclient.ExitCodeError)
	}

	_, err := snc.ReadConfiguration(files)
	if err != nil {
		snc.Log.Errorf("%s", err.Error())
		os.Exit(snclient.ExitCodeError)
	}

	snc.Log.Infof("OK - no configuration issues detected")
	os.Exit(snclient.ExitCodeOK)
}
