package commands

import (
	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/spf13/cobra"
)

func init() {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start the agent as a background daemon",
		Long: `Daemon mode starts the agent in the background.
The agent writes all logs to the configured log file.
`,
		GroupID: "daemon",
		Run: func(_ *cobra.Command, _ []string) {
			agentFlags.Mode = snclient.ModeServer
			snc := snclient.NewAgent(agentFlags)
			snc.CheckUpdateBinary("daemon")
			snc.RunBackground()
			snc.CleanExit(snclient.ExitCodeOK)
		},
	}
	addDaemonFlags(daemonCmd)
	rootCmd.AddCommand(daemonCmd)
}

func addDaemonFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&agentFlags.Pidfile, "pidfile", "", "", "Path to pid file")
}
