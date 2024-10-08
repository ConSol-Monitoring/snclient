package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

func init() {
	updateCmd := &cobra.Command{
		Use:   "update [channel|file]",
		Short: "Fetch and apply update",
		Long: `Check for updates. If check flag is set, no download will be
started, but the return will be set so the check can be used
as a check if there are updates available.

If channel is given, only this update channel will be checked. Otherwise all
configured and enabled channels.

Examples:

# check and apply updates from the configured release channels:
snclient update

# check for updates from all available channel including pre releases but do not download:
snclient update --prerelease --check all

# apply downgrade to version 0.19:
snclient update --downgrade=0.19
`,
		Run: runUpdates,
	}

	updateCmd.PersistentFlags().String("channel", "", "Select download channel.")
	updateCmd.PersistentFlags().Bool("check", false, "Check only, skip download.")
	updateCmd.PersistentFlags().BoolP("prerelease", "p", false, "Consider pre releases as well.")
	updateCmd.PersistentFlags().String("downgrade", "", "Force downgrade to given version.")
	updateCmd.PersistentFlags().BoolP("force", "f", false, "Force update.")
	rootCmd.AddCommand(updateCmd)
}

func runUpdates(cmd *cobra.Command, args []string) {
	agentFlags.Mode = snclient.ModeOneShot
	setInteractiveStdoutLogger()
	snc := snclient.NewAgent(agentFlags)
	executable := snclient.GlobalMacros["exe-full"]
	if strings.Contains(executable, ".update") || slices.Contains(args, "apply") {
		time.Sleep(500 * time.Millisecond)
		snc.CheckUpdateBinary("update")
		snc.CleanExit(0)
	}
	task := snc.Tasks.Get("Updates")
	mod, ok := task.(*snclient.UpdateHandler)
	if !ok {
		fmt.Fprintf(os.Stderr, "could not load update handler")
		snc.CleanExit(3)

		return
	}

	channel := ""
	if len(args) > 0 {
		channel = channel + "," + strings.Join(args, ",")
	} else {
		channel = cmd.Flag("channel").Value.String()
	}
	channel = strings.TrimPrefix(channel, ",")
	checkOnly := convert.Bool(cmd.Flag("check").Value.String())
	preRelease := convert.Bool(cmd.Flag("prerelease").Value.String())
	force := convert.Bool(cmd.Flag("force").Value.String())
	version, err := mod.CheckUpdates(
		context.TODO(),
		true,
		!checkOnly,
		false,
		preRelease,
		cmd.Flag("downgrade").Value.String(),
		channel,
		force,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update check failed: %s\n", err.Error())
		snc.CleanExit(3)
	}
	if version == "" {
		fmt.Fprintf(os.Stdout, "no new updates available (current version: %s - build: %s)\n", snc.Version(), snclient.Build)
		snc.CleanExit(0)
	}
	if checkOnly {
		fmt.Fprintf(os.Stdout, "new update available to version: %s\n", version)
		snc.CleanExit(1)
	}
	fmt.Fprintf(os.Stdout, "update to version %s applied successfully\n", version)
	snc.CleanExit(0)
}
