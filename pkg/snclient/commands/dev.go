package commands

import (
	"fmt"
	"os"

	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	devCmd := &cobra.Command{
		Use:   "dev",
		Short: "Collection of development commands",
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
		Short: "Watch main binary and config file for changes and restart automatically.",
		Long: `start the agent and watch for file changes in the config files or the agent itself.
The agent will be restarted immediately on file changes.
`,
		Run: func(_ *cobra.Command, _ []string) {
			agentFlags.Mode = snclient.ModeServer
			snc := snclient.NewAgent(agentFlags)
			snc.StartRestartWatcher()
			snc.Run()
		},
	})
}
