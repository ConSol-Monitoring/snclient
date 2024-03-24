package commands

import (
	"fmt"
	"math"
	"strings"

	"pkg/snclient"
	"pkg/utils"

	"github.com/reeflective/readline"
	"github.com/spf13/cobra"
)

func init() {
	testCmd := &cobra.Command{
		Use:     "test [cmd]",
		Aliases: []string{"run", "do"},
		Short:   "Start test mode or run given query",
		Long: `Test mode can be used to manually test queries.

If query is given a one shot result will be printed. Without command, snclient
will start a query prompt.

This command has aliases which slightly change behavior.

# human readable output
snclient test ...

# naemon/nagios (monitoring-plugins) plugin compatible output and exit code
snclient run ...

# do is an alias for run
snclient do ...

Examples:

# start query prompt:
snclient test

# run check_load and exit
snclient test check_load filter=none crit=none warn=none

# run check_memory with debug output
snclient test -vv check_memory

# run check_files directly as naemon check
snclient do check_files path=/tmp crit='count > 100'
`,
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			setInteractiveStdoutLogger()
			snc := snclient.NewAgent(agentFlags)

			if len(args) == 0 {
				if cmd.CalledAs() != "test" {
					cmd.Usage()

					snc.CleanExit(snclient.ExitCodeUnknown)
				}
				testPrompt(cmd, snc)

				return
			}
			rc := testRunCheck(cmd, snc, args)
			snc.CleanExit(rc)
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
	switch cmd.CalledAs() {
	case "test":
		testPrintHuman(cmd, res)
	case "do", "run":
		testPrintNaemon(cmd, res)
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

func testPrintHuman(cmd *cobra.Command, res *snclient.CheckResult) {
	fmt.Fprintf(rootCmd.OutOrStdout(), "Exit Code: %s (%d)\n", res.StateString(), res.State)
	fmt.Fprintf(rootCmd.OutOrStdout(), "Plugin Output:\n")
	fmt.Fprintf(rootCmd.OutOrStdout(), "%s\n", res.Output)
	if len(res.Metrics) > 0 {
		fmt.Fprintf(rootCmd.OutOrStdout(), "\nPerformance Metrics:\n")
		for _, m := range res.Metrics {
			fmt.Fprintf(rootCmd.OutOrStdout(), "  - %s\n", m.String())
		}
	}
}

func testPrintNaemon(cmd *cobra.Command, res *snclient.CheckResult) {
	output := string(res.BuildPluginOutput())
	output = strings.TrimSpace(output)
	fmt.Fprintf(rootCmd.OutOrStdout(), "%s\n", output)
}
