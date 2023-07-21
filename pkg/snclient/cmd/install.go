//go:build windows

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"pkg/snclient"
	"pkg/utils"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	// WINSERVICE set the service name for the windows service entry
	WINSERVICE = "snclient"

	// WINSERVICESTOPTIMEOUT sets the time to wait till a service is stopped
	WINSERVICESTOPTIMEOUT = 5 * time.Second

	// WINSERVICESTOPINTERVALL sets the interval at which the svc state is checked
	WINSERVICESTOPINTERVALL = 500 * time.Millisecond

	FIREWALLPREFIX = "SNClient"
)

var listenerNames = []string{"WEB", "NRPE", "Prometheus"}

func init() {
	installCmd := &cobra.Command{
		Use:   "install [cmd]",
		Short: "Install windows service and firewall exception",
		Long: `Install is used during msi installation for adding the windows service and a firewall exception.
It will also change some basic settings from the setup dialog. Ex. the initial password.
`,
	}
	rootCmd.AddCommand(installCmd)

	// install pkg
	installCmd.AddCommand(&cobra.Command{
		Use:    "pkg [args]",
		Short:  "called from the msi installer, set up firewall and service according to setup dialog",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			snc := snclient.NewAgent(agentFlags)

			installConfig := parseInstallerArgs(args)
			snc.Log.Infof("starting installer: %#v", installConfig)

			// merge tmp_installer.ini into snclient.ini
			err := mergeIniFile(snc, installConfig)
			if err != nil {
				snc.Log.Errorf("failed to write install ini: %s", err.Error())
			}

			// reload config
			snc.Init()

			if hasService(WINSERVICE) && serviceEnabled(WINSERVICE) {
				err := restartService(WINSERVICE)
				if err != nil {
					snc.Log.Errorf("failed to (re)start service: %s", err.Error())
				}
			}

			addFireWallRules(snc)
			snc.Log.Infof("installer finished successfully")
			os.Exit(0)
		},
	})

	// install firewall
	installCmd.AddCommand(&cobra.Command{
		Use:   "firewall [args]",
		Short: "add firewall exceptions for enabled tcp listeners, ex.: " + strings.Join(listenerNames, ", "),
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			snc := snclient.NewAgent(agentFlags)

			addFireWallRules(snc)
			snc.Log.Infof("firewall setup ready")
			os.Exit(0)
		},
	})
}

func parseInstallerArgs(args []string) (parsed map[string]string) {
	parsed = make(map[string]string, 0)
	if len(args) == 0 {
		return
	}

	for _, a := range strings.Split(args[0], "; ") {
		val := strings.SplitN(a, "=", 2)
		val[1] = strings.TrimSuffix(val[1], ";")
		parsed[val[0]] = val[1]
	}

	return parsed
}

func removeService(name string) error {
	if !hasService(name) {
		return nil
	}
	svcMgr, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer svcMgr.Disconnect()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return err
	}

	return windows.DeleteService(service.Handle)
}

func hasService(name string) bool {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer svcMgr.Disconnect()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return false
	}
	service.Close()

	return true
}

func serviceEnabled(name string) bool {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer svcMgr.Disconnect()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return false
	}
	defer service.Close()

	cfg, err := service.Config()
	if err != nil {
		return false
	}

	return cfg.StartType != windows.SERVICE_DISABLED
}

func stopService(name string) error {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer svcMgr.Disconnect()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return err
	}
	defer service.Close()

	state, err := service.Query()
	if state.State != svc.Stopped {
		state, err = service.Control(svc.Stop)
		if err != nil {
			return err
		}
	}

	if state.State == svc.Stopped {
		return nil
	}

	// Wait up to 10seconds for the service to stop
	startWait := time.Now()
	for state.State != svc.Stopped {
		time.Sleep(WINSERVICESTOPINTERVALL)
		state, err = service.Query()
		if err != nil {
			return err
		}
		if time.Now().After(startWait.Add(WINSERVICESTOPTIMEOUT)) {
			return fmt.Errorf("could not stop service within %s, current state: %v", WINSERVICESTOPTIMEOUT, state)
		}
	}

	return nil
}

func startService(name string) error {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer svcMgr.Disconnect()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return err
	}
	defer service.Close()

	err = service.Start()
	if err != nil {
		return err
	}

	return nil
}

func restartService(name string) error {
	err := stopService(name)
	if err != nil {
		return err
	}
	return startService(name)
}

func mergeIniFile(snc *snclient.Agent, installConfig map[string]string) error {
	installDir, ok := installConfig["INSTALLDIR"]
	if !ok {
		snc.Log.Errorf("no install dir found in arguments: %#v", installConfig)

		return nil
	}

	tmpFile := filepath.Join(installDir, "tmp_installer.ini")
	tmpConfig := snclient.NewConfig(false)
	file, err := os.Open(tmpFile)
	if err != nil {
		snc.Log.Debugf("failed to read %s: %s", tmpFile, err.Error())

		return nil
	}

	err = tmpConfig.ParseINI(file, tmpFile)
	if err != nil {
		snc.Log.Errorf("failed to parse %s: %s", tmpFile, err.Error())
	}
	file.Close()

	targetFile := filepath.Join(installDir, "snclient.ini")
	targetConfig := snclient.NewConfig(false)
	file, err = os.Open(targetFile)
	if err == nil {
		err = targetConfig.ParseINI(file, targetFile)
		if err != nil {
			snc.Log.Errorf("failed to read %s: %s", targetFile, err.Error())
		}
	}
	file.Close()

	for name, section := range tmpConfig.SectionsByPrefix("/") {
		targetSection := targetConfig.Section(name)
		for _, key := range section.Keys() {
			switch key {
			case "password":
				val, _ := section.GetString(key)
				if val != snclient.DefaultPassword {
					targetSection.Insert(key, toPassword(val))
				}
			case "use ssl", "WEBServer", "NRPEServer", "PrometheusServer":
				val, _ := section.GetString(key)
				targetSection.Insert(key, toBool(val))
			case "port", "allowed hosts":
				val, _ := section.GetString(key)
				targetSection.Insert(key, val)
			case "installer":
				val, _ := section.GetString(key)
				if val != "" {
					targetSection.Insert(key, val)
				}
			default:
				snc.Log.Errorf("unhandled merge ini key: %s", key)
			}
		}
	}

	err = targetConfig.WriteINI(targetFile)
	if err != nil {
		snc.Log.Errorf("failed to write %s: %s", targetFile, err.Error())
	}

	err = os.Remove(tmpFile)
	if err != nil {
		snc.Log.Errorf("failed to remove %s: %s", tmpFile, err.Error())
	}

	return nil
}

func toPassword(val string) string {
	if strings.HasPrefix(val, "SHA256:") {
		return val
	}

	sum, _ := utils.Sha256Sum(val)

	return fmt.Sprintf("%s:%s", "SHA256", sum)
}

func toBool(val string) string {
	switch val {
	case "1":
		return "enabled"
	default:
		return "disabled"
	}
}

func addFireWallRules(snc *snclient.Agent) {
	for _, name := range listenerNames {
		enabled, _, err := snc.Config.Section("/modules").GetBool(name + "Server")
		if err != nil {
			snc.Log.Debugf("/modules/%sServer: %s", name, err.Error())

			continue
		}
		if !enabled {
			continue
		}

		port, ok, err := snc.Config.Section("/settings/" + name + "/server").GetInt("port")
		if err != nil {
			snc.Log.Debugf("/settings/%s/server/port: %s", name, err.Error())

			continue
		}
		if ok {
			err := addFireWallRule(snc, name, port)
			if err != nil {
				snc.Log.Errorf("addFireWallRule: %s%s: %s", FIREWALLPREFIX, name, err.Error())
			}
		}
	}
}

func addFireWallRule(snc *snclient.Agent, name string, port int64) error {
	removeFireWallRule(snc, name)
	snc.Log.Debugf("adding firewall rule '%s%s' for port: %d", FIREWALLPREFIX, name, port)
	os.Chdir("C:\\") // avoid: exec: "netsh": cannot run executable found relative to current directory
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"dir=in",
		"action=allow",
		"protocol=TCP",
		fmt.Sprintf("localport=%d", port),
		fmt.Sprintf("name=%s%s", FIREWALLPREFIX, name),
	)
	pwd, _ := os.Getwd()
	snc.Log.Errorf("pwd: %s", pwd)

	output, err := cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("Failed to create firewall exception: %s (%s)", err.Error(), output)
	}

	snc.Log.Debugf("added firewall: %s", output)

	return nil
}

func removeFireWallRules(snc *snclient.Agent) {
	for _, name := range listenerNames {
		err := removeFireWallRule(snc, name)
		if err != nil {
			snc.Log.Errorf("removeFireWallRule: %s%s: %s", FIREWALLPREFIX, name, err.Error())
		}
	}
}

func removeFireWallRule(snc *snclient.Agent, name string) error {
	snc.Log.Debugf("removing firewall rule '%s%s'", FIREWALLPREFIX, name)
	os.Chdir("C:\\") // avoid: exec: "netsh": cannot run executable found relative to current directory
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s%s", FIREWALLPREFIX, name),
	)

	output, err := cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("Failed to remove firewall exception: %s (%s)", err.Error(), output)
	}

	snc.Log.Debugf("removed firewall: %s", output)

	return nil
}
