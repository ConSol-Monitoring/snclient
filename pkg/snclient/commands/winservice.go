//go:build windows

package commands

import (
	"os"

	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	winSvcCmd := &cobra.Command{
		Use:   "winservice",
		Short: "Start the agent from a windows service",
		Long: `winservice mode starts as windows service.
All logs will be written to the configured logfile.
`,
		GroupID: "daemon",
		Run: func(cmd *cobra.Command, _ []string) {
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
