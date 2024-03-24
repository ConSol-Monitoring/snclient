package commands

import (
	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the agent",
		Long: `server mode starts the agent in foreground connected to the terminal.
All logs will be printed to stdout unless flags tell otherwise.
		`,
		GroupID: "daemon",
		Run: func(cmd *cobra.Command, _ []string) {
			agentFlags.Mode = snclient.ModeServer
			snc := snclient.NewAgent(agentFlags)
			snc.CheckUpdateBinary("server")
			snc.Run()
			snc.CleanExit(snclient.ExitCodeOK)
		},
	}
	addDaemonFlags(serverCmd)
	rootCmd.AddCommand(serverCmd)
}
