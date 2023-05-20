package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Fetch and apply update",
		Run: func(cmd *cobra.Command, args []string) {
			// TODO:...
			fmt.Printf("update\n")
		},
	})
}
