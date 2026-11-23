package commands

import (
	"fmt"
	"math"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/snclient"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/reeflective/readline"
	"github.com/spf13/cobra"
)

func init() {
	testCmd := &cobra.Command{
		Use:   "test [cmd]",
		Short: "Start test mode or run a query",
		Long: `Use test mode to test queries manually.

If you provide a query, SNClient prints its result once. Without a command, SNClient starts a query prompt.

test and run use different startup paths and output formats.
Both load configuration, aliases, and external scripts. Neither starts listeners.
test initializes configured tasks and starts the memory watcher if enabled.
run executes one check without background task initialization or the memory watcher.

# human readable output
snclient test ...

# naemon/nagios (monitoring-plugins) plugin compatible output and exit code
snclient run ...

Examples:

# start query prompt:
snclient test

# run check_load and exit
snclient test check_load filter=none crit=none warn=none

# run check_memory with debug output
snclient test -vv check_memory

# run check_files directly as naemon check
snclient run check_files path=/tmp crit='count > 100'

# run an HTTP check without background tasks
snclient run check_http -H localhost -p 443 -S
`,
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			agentFlags.SkipBackgroundTasks = cmd.CalledAs() != "test"
			setInteractiveStdoutLogger()
			snc := snclient.NewAgent(agentFlags)
			if !agentFlags.SkipBackgroundTasks {
				_ = snc.StartTask("ProcessMemoryWatcher")
			}

			if len(args) == 0 {
				if cmd.CalledAs() != "test" {
					_ = cmd.Usage()

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

	runCmd := &cobra.Command{
		Use:   "run <cmd> [args]",
		Short: "Run one check without background tasks",
		Long: `Run one check and return plugin output and its exit code.
SNClient loads configuration, aliases, and external scripts.
It skips background task initialization and the memory watcher. It does not start listeners.`,
		Example: "  snclient run check_http -H localhost -p 443 -S",
		Run:     testCmd.Run,
	}
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:    "do <cmd> [args]",
		Hidden: true,
		Long:   runCmd.Long,
		Run:    runCmd.Run,
	})
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
	_ = rl.Config.Set("show-mode-in-prompt", false)
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

func testPrintHuman(_ *cobra.Command, res *snclient.CheckResult) {
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

func testPrintNaemon(_ *cobra.Command, res *snclient.CheckResult) {
	output := string(res.BuildPluginOutput())
	output = strings.TrimSpace(output)
	fmt.Fprintf(rootCmd.OutOrStdout(), "%s\n", output)
}
