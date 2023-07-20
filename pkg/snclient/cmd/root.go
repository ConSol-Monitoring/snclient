package cmd

import (
	"fmt"
	"os"
	"strings"

	"pkg/snclient"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var agentFlags = &snclient.AgentFlags{}

var rootCmd = NewRootCmd()

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snclient [global flags] [command]",
		Short: "Multi-platform monitoring agent for Naemon and Prometheus.",
		Long: `SNClient+ is a generic monitoring agent available for multiple platforms.
It aims to provide a basic set of fault monitoring and metrics
while being easily extendible with own script and checks.`,
		Example: `  * Start server
    %> snclient server

  * Start as daemon in background
    %> snclient daemon

  * Check for update in verbose mode
    %> snclient update -v
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// defaults to server mode unless --help/--version is given
			if agentFlags.Version {
				snc := snclient.Agent{}
				snc.PrintVersion()
				return nil
			}

			if agentFlags.Help {
				return nil
			}

			// should never reach this point
			return fmt.Errorf("snclient called without arguments, see --help for usage.")
		},
	}

	return cmd
}

func init() {
	addFlags(rootCmd, agentFlags)
}

func addFlags(cmd *cobra.Command, flags *snclient.AgentFlags) {
	cmd.PersistentFlags().BoolVarP(&flags.Help, "help", "h", false, "print help and exit")
	cmd.PersistentFlags().BoolVarP(&flags.Version, "version", "V", false, "print version and exit")
	cmd.PersistentFlags().StringArrayVarP(&flags.ConfigFiles, "config", "c", []string{}, "path to config file, supports wildcards like *.ini (default is ./snclient.ini) (multiple)")
	cmd.PersistentFlags().BoolVarP(&flags.Quiet, "quiet", "q", false, "set loglevel to error")
	cmd.PersistentFlags().CountVarP(&flags.Verbose, "verbose", "v", "increase loglevel, -v means debug, -vv means trace")
	cmd.PersistentFlags().StringVarP(&flags.LogLevel, "loglevel", "", "", "set loglevel to one of: off, error, info, debug, trace")
	cmd.PersistentFlags().StringVarP(&flags.LogFormat, "logformat", "", "", "override logformat, see https://pkg.go.dev/github.com/kdar/factorlog")
	cmd.PersistentFlags().StringVarP(&flags.LogFile, "logfile", "", "", "Path to log file or stdout/stderr")
	cmd.PersistentFlags().StringVarP(&flags.ProfilePort, "debug-profiler", "", "", "start pprof profiler on this port, ex. :6060")
	cmd.PersistentFlags().StringVarP(&flags.ProfileCPU, "cpuprofile", "", "", "write cpu profile to `file")
	cmd.PersistentFlags().StringVarP(&flags.ProfileMem, "memprofile", "", "", "write memory profile to `file")
	cmd.PersistentFlags().IntVarP(&flags.DeadlockTimeout, "debug-deadlock", "", 0, "enable deadlock detection with given timeout")
	cmd.PersistentFlags().MarkHidden("debug-deadlock") // there are no lock so far

	cmd.DisableAutoGenTag = true
	cmd.DisableSuggestions = true

	cmd.PersistentFlags().SortFlags = false
	cmd.Flags().SortFlags = false

	cmd.AddGroup(&cobra.Group{ID: "daemon", Title: "Server commands:"})
	cmd.SetUsageTemplate(usageTemplate)
}

func Execute() error {
	sanitizeOSArgs()
	maybeInjectRootAlias(rootCmd, "server")
	return rootCmd.Execute()
}

func maybeInjectRootAlias(rootCmd *cobra.Command, inject string) {
	cmd, args, err := rootCmd.Find(os.Args[1:])
	if err != nil {
		return
	}

	// are we going for the root command?
	if cmd.Name() != rootCmd.Name() {
		return
	}

	tmpFlags := &snclient.AgentFlags{}
	tmpCmd := NewRootCmd()
	addFlags(tmpCmd, tmpFlags)

	// parse flags (ignoring unknown flags for subcommands) and check if we want help or version only
	tmpCmd.FParseErrWhitelist.UnknownFlags = true
	tmpCmd.ParseFlags(args)
	if tmpFlags.Version {
		os.Args = []string{os.Args[0], "-V"}
		return
	}
	if tmpFlags.Help {
		os.Args = []string{os.Args[0], "-h"}
		return
	}

	os.Args = append([]string{os.Args[0], inject}, os.Args[1:]...)
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
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
