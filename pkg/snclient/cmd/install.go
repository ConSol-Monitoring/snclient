//go:build windows

package cmd

import (
	"fmt"
	"os"
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
		Use:   "pkg [args]",
		Short: "called from the msi installer, set up firewall and service according to setup dialog",
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			snc := snclient.NewAgent(agentFlags)

			installConfig := parseInstallerArgs(args)

			if installConfig["WIX_UPGRADE_DETECTED"] == "" {
				snc.Log.Infof("starting installer: %#v", installConfig)
			}

			if hasService("snclient") {
				err := restartService(WINSERVICE)
				if err != nil {
					snc.Log.Errorf("failed to (re)start service: %s", err.Error())
				}
			}

			snc.Log.Infof("installer finished successfully")
			os.Exit(0)
		},
	})

	// install set
	installCmd.AddCommand(&cobra.Command{
		Use:   "set [args]",
		Short: "called from the msi installer, set up configuration according to setup dialog",
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			snc := snclient.NewAgent(agentFlags)

			installConfig := parseInstallerArgs(args)

			if installConfig["WIX_UPGRADE_DETECTED"] != "" {
				return
			}

			snc.Log.Infof("installer set: %#v", installConfig)
			err := writeIniFile(snc, installConfig)
			if err != nil {
				snc.Log.Errorf("failed to write install ini: %s", err.Error())
			}

			snc.Log.Infof("installer finished successfully")
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

func writeIniFile(snc *snclient.Agent, installConfig map[string]string) error {
	installDir, ok := installConfig["INSTALLDIR"]
	if !ok {
		snc.Log.Errorf("no install dir found in arguments: %#v", installConfig)

		return nil
	}
	targetFile := filepath.Join(installDir, "snclient_local.ini")
	config := snclient.NewConfig()
	file, err := os.Open(targetFile)
	if err == nil {
		err = config.ParseINI(file, targetFile, false)
		if err != nil {
			snc.Log.Errorf("failed to read %s: %s", targetFile, err.Error())
		}
	}

	for key, value := range installConfig {
		if value == "" {
			continue
		}
		switch key {
		case "INSTALLDIR", "WIX_UPGRADE_DETECTED":
		case "PASSWORD":
			if value != snclient.DefaultPassword {
				config.Section("/settings/WEB/server").Set("password", toPassword(value))
			}
		case "ALLOWEDHOSTS":
			config.Section("/settings/default").Set("allowed hosts", value)
		case "INCLUDES":
			config.Section("/includes").Set("installer", value)
		case "WEBSERVER":
			config.Section("/modules").Set("WEBServer", toBool(value))
		case "WEBSERVERPORT":
			config.Section("/settings/WEB/server").Set("port", value)
		case "WEBSERVERSSL":
			config.Section("/settings/WEB/server").Set("use ssl", toBool(value))
		case "NRPESERVER":
			config.Section("/modules").Set("NRPEServer", toBool(value))
		case "NRPESERVERPORT":
			config.Section("/settings/NRPE/server").Set("port", value)
		case "NRPESERVERSSL":
			config.Section("/settings/NRPE/server").Set("use ssl", toBool(value))
		case "PROMETHEUSSERVER":
			config.Section("/modules").Set("PrometheusServer", toBool(value))
		case "PROMETHEUSSERVERPORT":
			config.Section("/settings/Prometheus/server").Set("port", value)
		case "PROMETHEUSSERVERSSL":
			config.Section("/settings/Prometheus/server").Set("use ssl", toBool(value))
		default:
			snc.Log.Errorf("unknown config attribute: %s = %s", key, value)
		}
	}

	err = config.WriteINI(targetFile)
	if err != nil {
		snc.Log.Errorf("failed to write %s: %s", targetFile, err.Error())
	}

	return nil
}

func toPassword(val string) string {
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
