package cmd

import (
	"os"

	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	uninstallCmd := &cobra.Command{
		Use:   "uninstall [cmd]",
		Short: "Uninstall windows service and firewall exception",
		Long: `Uninstall is used during msi installation for removing the windows service and a firewall exception.
`,
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			snc := snclient.NewAgent(agentFlags)

			installConfig := parseInstallerArgs(args[0])
			snc.Log.Errorf("****** uninstall: %#v", installConfig)
			err := stopService("snclient")
			if err != nil {
				snc.Log.Errorf("failed to stops service: %s", err.Error())
			}
			err = removeService("snclient")
			if err != nil {
				snc.Log.Errorf("failed to remove service: %s", err.Error())
			}
			os.Exit(0)
		},
	}
	rootCmd.AddCommand(uninstallCmd)
}
