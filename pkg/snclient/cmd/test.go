package cmd

import (
	"fmt"
	"math"
	"os"
	"strings"

	"pkg/snclient"
	"pkg/utils"

	"github.com/reeflective/readline"
	"github.com/spf13/cobra"
)

func init() {
	testCmd := &cobra.Command{
		Use:   "test [cmd]",
		Short: "Start test mode or run given query",
		Long: `Test mode can be used to manually test queries.

If query is given a one shot result will be printed. Without command, snclient
will start a query prompt.

Examples:

# start query prompt:
snclient test

# run check_load and exit
snclient test check_load filter=none crit=none warn=none

# run check_memory with debug output
snclient test -vv check_memory
`,
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.LogFile = "stdout"
			agentFlags.LogFormat = snclient.LogColors + `[%{Time "15:04:05.000"}][%{S}] %{Message}` + snclient.LogColorReset
			if agentFlags.Verbose > 2 {
				agentFlags.LogFormat = snclient.LogColors + `[%{Time "15:04:05.000"}][%{S}][%{ShortFile}:%{Line}] %{Message}` + snclient.LogColorReset
			}
			snc := snclient.NewAgent(agentFlags)
			if len(args) == 0 {
				testPrompt(cmd, snc)

				return
			}
			rc := testRunCheck(cmd, snc, args)
			os.Exit(rc)
		},
	}
	rootCmd.AddCommand(testCmd)
}

func testPrompt(cmd *cobra.Command, snc *snclient.Agent) {
	promptCompleter := func(line []rune, cursor int) readline.Completions {
		checks := []string{}
		filter := string(line[0:cursor])
		for chk := range snclient.AvailableChecks {
			if strings.HasPrefix(chk, filter) {
				checks = append(checks, chk)
			}
		}
		return readline.CompleteValues(checks...)
	}

	snc.PrintVersion()
	fmt.Fprintf(rootCmd.OutOrStdout(), "enter check command, 'help' or 'exit'.\n")

	rl := readline.NewShell()
	rl.Prompt.Primary(func() string { return ">> " })
	rl.Config.Set("show-mode-in-prompt", false)
	rl.Completer = promptCompleter
	for {
		text, err := rl.Readline()
		if err != nil {
			return
		}
		switch text {
		case "exit":
			return
		case "":
			break
		case "help":
			testHelp(cmd)
		default:
			args := utils.Tokenize(text)
			testRunCheck(cmd, snc, args)
		}
	}
}

func testRunCheck(cmd *cobra.Command, snc *snclient.Agent, args []string) int {
	res := snc.RunCheck(args[0], args[1:])
	fmt.Fprintf(rootCmd.OutOrStdout(), "Exit Code: %s (%d)\n", res.StateString(), res.State)
	fmt.Fprintf(rootCmd.OutOrStdout(), "Plugin Output:\n")
	fmt.Fprintf(rootCmd.OutOrStdout(), "%s\n", res.Output)
	if len(res.Metrics) > 0 {
		fmt.Fprintf(rootCmd.OutOrStdout(), "\nPerformance Metrics:\n")
		for _, m := range res.Metrics {
			fmt.Fprintf(rootCmd.OutOrStdout(), "  - %s\n", m.String())
		}
	}

	state := int(3)
	if res.State >= 0 && res.State <= math.MaxInt {
		state = int(res.State)
	}

	return state
}

func testHelp(cmd *cobra.Command) {
	fmt.Fprintf(rootCmd.OutOrStdout(), "%s", cmd.Long)
}
