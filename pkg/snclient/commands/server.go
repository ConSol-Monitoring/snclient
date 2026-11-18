package commands

import (
	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/spf13/cobra"
)

func init() {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the agent (default)",
		Long: `Server mode starts the agent in the foreground, connected to the terminal.
The agent prints all logs to stdout unless flags specify another destination.
This is the default mode when you omit the command.
		`,
		GroupID: "daemon",
		Run: func(_ *cobra.Command, _ []string) {
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
