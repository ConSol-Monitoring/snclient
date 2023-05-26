package cmd

import (
	"fmt"
	"os"
	"strings"

	"pkg/convert"
	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	updateCmd := &cobra.Command{
		Use:   "update [channel]",
		Short: "Fetch and apply update",
		Long: `Check for updates. If check flag is set, no download will be
started, but the return will be set so the check can be used
as a check if there are updates available.

If channel is given, only this update channel will be checked. Otherwise all
configured and enabled channels.

Examples:

# check and apply updates from the stable release channel
snclient update

# check for updates from all available channel including pre releases but do not download.
snclient update --prerelease --check all

# apply downgrade to version 0.2.
snclient update --downgrade=0.2
`,
		Run: func(cmd *cobra.Command, args []string) {
			snc := snclient.NewAgent(agentFlags)
			task := snc.Tasks.Get("Updates")
			switch mod := task.(type) {
			case *snclient.UpdateHandler:
				channel := ""
				if len(args) > 0 {
					channel = channel + "," + strings.Join(args, ",")
				} else {
					channel = cmd.Flag("channel").Value.String()
				}
				channel = strings.TrimPrefix(channel, ",")
				checkOnly := convert.Bool(cmd.Flag("check").Value.String())
				preRelease := convert.Bool(cmd.Flag("prerelease").Value.String())
				version, err := mod.CheckUpdates(
					true,
					!checkOnly,
					false,
					preRelease,
					cmd.Flag("downgrade").Value.String(),
					channel,
				)
				if err != nil {
					fmt.Fprintf(os.Stderr, "update check failed: %s\n", err.Error())
					os.Exit(3)
				}
				if version == "" {
					fmt.Fprintf(os.Stdout, "no new updates available\n")
					os.Exit(0)
				}
				if checkOnly {
					fmt.Fprintf(os.Stdout, "new update available to version: %s\n", version)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stdout, "update to version %s applied successfully\n", version)
				os.Exit(0)
			}
		},
	}

	updateCmd.PersistentFlags().String("channel", "stable", "Select download channel.")
	updateCmd.PersistentFlags().Bool("check", false, "Check only, skip download.")
	updateCmd.PersistentFlags().BoolP("prerelease", "p", false, "Consider pre releases as well.")
	updateCmd.PersistentFlags().String("downgrade", "", "Force downgrade to given version.")
	rootCmd.AddCommand(updateCmd)
}
