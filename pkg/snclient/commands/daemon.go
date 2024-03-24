package commands

import (
	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start the agent demonized in background",
		Long: `daemon mode starts the agent in background.
All logs will be written to the configured logfile.
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
