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

			if hasService("snclient") {
				err := restartService(WINSERVICE)
				if err != nil {
					snc.Log.Errorf("failed to (re)start service: %s", err.Error())
				}
			}

			err = addFireWallRules(snc)
			if err != nil {
				snc.Log.Errorf("failed to setup firewall: %s", err.Error())
			}

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

			err := addFireWallRules(snc)
			if err != nil {
				snc.Log.Errorf("failed to setup firewall: %s", err.Error())
			}

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
	tmpConfig := snclient.NewConfig()
	file, err := os.Open(tmpFile)
	if err != nil {
		snc.Log.Debugf("failed to read %s: %s", tmpFile, err.Error())

		return nil
	}

	err = tmpConfig.ParseINI(file, tmpFile, false)
	if err != nil {
		snc.Log.Errorf("failed to parse %s: %s", tmpFile, err.Error())
	}

	targetFile := filepath.Join(installDir, "snclient.ini")
	targetConfig := snclient.NewConfig()
	file, err = os.Open(targetFile)
	if err == nil {
		err = targetConfig.ParseINI(file, targetFile, false)
		if err != nil {
			snc.Log.Errorf("failed to read %s: %s", targetFile, err.Error())
		}
	}

	for name, section := range tmpConfig.SectionsByPrefix("/") {
		targetSection := targetConfig.Section(name)
		for _, key := range section.Keys() {
			switch key {
			case "password":
				val, _ := section.GetString(key)
				if val != snclient.DefaultPassword {
					targetSection.Set(key, toPassword(val))
				}
			case "use ssl", "WEBServer", "NRPEServer", "PROMETHEUSServer":
				val, _ := section.GetString(key)
				targetSection.Set(key, toBool(val))
			case "installer", "port", "allowed hosts":
				val, _ := section.GetString(key)
				targetSection.Set(key, val)
			default:
				snc.Log.Errorf("unhandled merge ini key: %s", key)
			}
		}
	}

	err = targetConfig.WriteINI(targetFile)
	if err != nil {
		snc.Log.Errorf("failed to write %s: %s", targetFile, err.Error())
	}

	os.Remove(tmpFile)

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

func addFireWallRules(snc *snclient.Agent) error {
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
				snc.Log.Debugf("addFireWallRule: %s", name, err.Error())
			}
		}
	}
	return nil
}

func addFireWallRule(snc *snclient.Agent, name string, port int64) error {
	removeFireWallRule(snc, name)
	snc.Log.Debugf("adding firewall rule 'SNClient%s' for port: %d", name, port)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"dir=in",
		"action=allow",
		"protocol=TCP",
		fmt.Sprintf("localport=%d", port),
		fmt.Sprintf("name=SNClient%s", name),
	)

	output, err := cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("Failed to create firewall exception: %s (%s)", err.Error(), output)
	}

	snc.Log.Debugf("added firewall: %s", output)

	return nil
}

func removeFireWallRules(snc *snclient.Agent) error {
	for _, name := range listenerNames {
		err := removeFireWallRule(snc, name)
		if err != nil {
			snc.Log.Debugf("removeFireWallRule: %s", name, err.Error())
		}
	}

	return nil
}

func removeFireWallRule(snc *snclient.Agent, name string) error {
	snc.Log.Debugf("removing firewall rule 'SNClient%s'", name)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=SNClient%s", name),
	)

	output, err := cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("Failed to remove firewall exception: %s (%s)", err.Error(), output)
	}

	snc.Log.Debugf("removed firewall: %s", output)

	return nil
}
