//go:build windows

package commands

import (
	"os"

	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/spf13/cobra"
)

func init() {
	winSvcCmd := &cobra.Command{
		Use:   "winservice",
		Short: "Start the agent from a Windows service",
		Long: `Winservice mode starts the agent as a Windows service.
The agent writes all logs to the configured log file.
`,
		GroupID: "daemon",
		Run: func(_ *cobra.Command, _ []string) {
			agentFlags.Mode = snclient.ModeServer
			snc := snclient.NewAgent(agentFlags)
			snc.CheckUpdateBinary("winservice")
			snc.RunAsWinService()
			snc.Log.Infof("snclient exited (pid:%d)\n", os.Getpid())
			snc.CleanExit(snclient.ExitCodeOK)
		},
		Hidden: true,
	}
	addDaemonFlags(winSvcCmd)
	rootCmd.AddCommand(winSvcCmd)
}
