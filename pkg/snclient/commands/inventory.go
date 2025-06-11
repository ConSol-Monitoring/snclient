package commands

import (
	"context"
	"fmt"

	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/goccy/go-json"
	"github.com/spf13/cobra"
)

func init() {
	invCmd := &cobra.Command{
		Use:     "inventory [<module>]",
		Aliases: []string{"inv"},
		Short:   "Gather inventory and print as json strucure",
		Long: `Inventory returns the same output as the rest path /api/v1/inventory

# print inventory
snclient inventory

# print inventory for mounts only
snclient inventory mounts
`,
		Run: func(_ *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			setInteractiveStdoutLogger()
			snc := snclient.NewAgent(agentFlags)

			inventory := snc.GetInventory(context.Background(), args)
			encoder := json.NewEncoder(rootCmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			err := encoder.Encode(inventory)
			if err != nil {
				fmt.Fprintf(rootCmd.OutOrStderr(), "ERROR: %s\n", err.Error())
				snc.CleanExit(1)
			}

			snc.CleanExit(0)
		},
	}
	rootCmd.AddCommand(invCmd)
}
