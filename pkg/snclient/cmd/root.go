package cmd

import (
	"fmt"
	"os"
	"strings"

	"pkg/snclient"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var rootCmd = &cobra.Command{
	Use:   "snclient [global flags] [command]",
	Short: "Multi-platform monitoring agent for Naemon and Prometheus.",
	Long: `SNClient+ is a generic monitoring agent available for multiple platforms.
It aims to provide a basic set of fault monitoring and metrics
while being easily extendible with own script and checks.`,
	Run: func(cmd *cobra.Command, args []string) {
		// default to server mode
		// should never reach this point
		fmt.Fprintf(os.Stderr, "snclient called without arguments, see --help for usage.")
		os.Exit(3)
	},
	PreRun: func(cmd *cobra.Command, _ []string) {
		if agentFlags.Version {
			snc := snclient.NewAgent(agentFlags)
			snc.PrintVersion()
			os.Exit(snclient.ExitCodeOK)
		}
	},
}

var agentFlags = &snclient.AgentFlags{}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&agentFlags.Help, "help", "h", false, "print help and exit")
	rootCmd.PersistentFlags().BoolVarP(&agentFlags.Version, "version", "V", false, "print version and exit")
	rootCmd.PersistentFlags().StringArrayVarP(&agentFlags.ConfigFiles, "config", "c", []string{}, "path to config file, supports wildcards like *.ini (default is ./snclient.ini) (multiple)")
	rootCmd.PersistentFlags().BoolVarP(&agentFlags.Quiet, "quiet", "q", false, "set loglevel to error")
	rootCmd.PersistentFlags().CountVarP(&agentFlags.Verbose, "verbose", "v", "increase loglevel, -v means debug, -vv means trace")
	rootCmd.PersistentFlags().StringVarP(&agentFlags.LogLevel, "loglevel", "", "info", "set loglevel to one of: off, error, info, debug, trace")
	rootCmd.PersistentFlags().StringVarP(&agentFlags.LogFormat, "logformat", "", "", "override logformat, see https://pkg.go.dev/github.com/kdar/factorlog")
	rootCmd.PersistentFlags().StringVarP(&agentFlags.LogFile, "logfile", "", "", "Path to log file or stdout/stderr")
	rootCmd.PersistentFlags().StringVarP(&agentFlags.ProfilePort, "debug-profiler", "", "", "start pprof profiler on this port, ex. :6060")
	rootCmd.PersistentFlags().StringVarP(&agentFlags.ProfileCPU, "cpuprofile", "", "", "write cpu profile to `file")
	rootCmd.PersistentFlags().StringVarP(&agentFlags.ProfileMem, "memprofile", "", "", "write memory profile to `file")
	rootCmd.PersistentFlags().IntVarP(&agentFlags.DeadlockTimeout, "debug-deadlock", "", 0, "enable deadlock detection with given timeout")
	rootCmd.PersistentFlags().MarkHidden("debug-deadlock") // there are no lock so far

	rootCmd.DisableAutoGenTag = true
	rootCmd.DisableSuggestions = true

	rootCmd.PersistentFlags().SortFlags = false
	rootCmd.Flags().SortFlags = false

	rootCmd.AddGroup(&cobra.Group{ID: "daemon", Title: "Server commands:"})
	rootCmd.SetUsageTemplate(usageTemplate)
}

func Execute() error {
	sanitizeOSArgs()
	maybeInjectRootAlias(rootCmd, "server")
	return rootCmd.Execute()
}

func maybeInjectRootAlias(rootCmd *cobra.Command, inject string) {
	nonRootAliases := nonRootSubCmds(rootCmd)

	if len(os.Args) > 1 {
		for _, v := range nonRootAliases {
			if os.Args[1] == v {
				return
			}
		}
	}
	os.Args = append([]string{os.Args[0], inject}, os.Args[1:]...)
}

func nonRootSubCmds(rootCmd *cobra.Command) []string {
	res := []string{"help", "completion", "-h", "--help", "-V", "--version"}
	for _, c := range rootCmd.Commands() {
		res = append(res, c.Name())
		res = append(res, c.Aliases...)
	}

	return res
}

func sanitizeOSArgs() {
	// sanitize some args
	replace := map[string]string{}
	rootCmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "" {
			replace["-"+f.Name] = "--" + f.Name
		}
	})
	for _, c := range rootCmd.Commands() {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name != "" {
				replace["-"+f.Name] = "--" + f.Name
			}
		})
	}
	for i, a := range os.Args {
		if i == 0 {
			continue
		}
		if r, ok := replace[a]; ok {
			os.Args[i] = r
		}
		for n, r := range replace {
			if strings.HasPrefix(a, n+"=") {
				os.Args[i] = r + "=" + strings.TrimPrefix(os.Args[i], n+"=")
			}
		}
	}
}

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
