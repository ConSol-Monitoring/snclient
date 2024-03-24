//go:build windows

package commands

import (
	"os"
	"path/filepath"

	"pkg/snclient"

	"github.com/spf13/cobra"
)

func init() {
	uninstallCmd := &cobra.Command{
		Use:   "uninstall [cmd]",
		Short: "Uninstall windows service and firewall exception",
		Long: `Uninstall is used during msi installation for removing the windows service and a firewall exception.
`,
	}
	rootCmd.AddCommand(uninstallCmd)

	// uninstall stop
	uninstallCmd.AddCommand(&cobra.Command{
		Use:   "stop [args]",
		Short: "called from the msi installer stop the service",
		Run:   uninstallStop,
	})

	// uninstall pkg
	uninstallCmd.AddCommand(&cobra.Command{
		Use:    "pkg [args]",
		Short:  "called from the msi installer, removes firewall and service if agent gets uninstalled",
		Hidden: true,
		Run:    uninstallPkg,
	})

	// uninstall firewall
	uninstallCmd.AddCommand(&cobra.Command{
		Use:    "firewall [args]",
		Short:  "remove existing firewall rules",
		Hidden: true,
		Run:    uninstallFirewall,
	})
}

func uninstallStop(_ *cobra.Command, _ []string) {
	agentFlags.Mode = snclient.ModeOneShot
	snc := snclient.NewAgent(agentFlags)

	snc.Log.Infof("uninstaller: stop")
	if hasService(WINSERVICE) {
		err := stopService(WINSERVICE)
		if err != nil {
			snc.Log.Errorf("failed to stops service: %s", err.Error())
		}
	}
	snc.Log.Infof("stop completed")

	err := removeService(WINSERVICE)
	if err != nil {
		snc.Log.Errorf("failed to remove service: %s", err.Error())
	}

	snc.CleanExit(0)
}

func uninstallPkg(_ *cobra.Command, args []string) {
	agentFlags.Mode = snclient.ModeOneShot
	snc := snclient.NewAgent(agentFlags)

	installConfig := parseInstallerArgs(args)
	if installConfig["REMOVE"] != "ALL" || installConfig["UPGRADINGPRODUCTCODE"] != "" {
		snc.Log.Infof("skipping uninstall: %#v", installConfig)
		snc.CleanExit(0)
	}

	snc.Log.Infof("starting uninstaller: %#v", installConfig)
	if hasService(WINSERVICE) {
		err := stopService("snclient")
		if err != nil {
			snc.Log.Errorf("failed to stops service: %s", err.Error())
		}
		err = removeService("snclient")
		if err != nil {
			snc.Log.Errorf("failed to remove service: %s", err.Error())
		}
	}

	// cleanup windows_exporter textfile_inputs folder
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "exporter", "textfile_inputs"))
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "exporter"))

	removeFireWallRules(snc)

	snc.Log.Infof("uninstall completed")

	// close log file so we can delete it
	snc.Log.SetOutput(os.Stderr)
	if snclient.LogFileHandle != nil {
		snclient.LogFileHandle.Close()
		snclient.LogFileHandle = nil
	}

	// since files are installed with Permanent=yes, we need to remove them manually now
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "cacert.pem"))
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "server.crt"))
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "server.key"))
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "snclient.ini"))
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "snclient.log"))
	_ = os.Remove(filepath.Join(installConfig["INSTALLDIR"], "snclient.log.old"))

	snc.CleanExit(0)
}

func uninstallFirewall(_ *cobra.Command, _ []string) {
	agentFlags.Mode = snclient.ModeOneShot
	snc := snclient.NewAgent(agentFlags)

	removeFireWallRules(snc)

	snc.Log.Infof("firewall exceptions removed")
	snc.CleanExit(0)
}
