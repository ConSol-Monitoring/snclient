package commands

import (
	"fmt"
	"os"

	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/spf13/cobra"
)

func init() {
	devCmd := &cobra.Command{
		Use:   "dev",
		Short: "Run development commands",
		Run: func(_ *cobra.Command, _ []string) {
			rootCmd.SetArgs([]string{"help", "dev"})
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "command failed: %s", err.Error())
				os.Exit(3)
			}
		},
		Hidden: true,
	}
	rootCmd.AddCommand(devCmd)

	// dev watch
	devCmd.AddCommand(&cobra.Command{
		Use:   "watch",
		Short: "Restart the agent automatically when its binary or configuration changes.",
		Long: `Start the agent and watch its binary and configuration files for changes.
Restart the agent immediately when these files change.
`,
		Run: func(_ *cobra.Command, _ []string) {
			agentFlags.Mode = snclient.ModeServer
			snc := snclient.NewAgent(agentFlags)
			snc.StartRestartWatcher()
			snc.Run()
		},
	})
}
