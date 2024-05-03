//go:build windows

package commands

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
		Run:    installPkg,
	})

	// install pre
	installCmd.AddCommand(&cobra.Command{
		Use:    "pre [args]",
		Short:  "called from the msi installer, stop services",
		Hidden: true,
		Run:    installPre,
	})

	// install firewall
	installCmd.AddCommand(&cobra.Command{
		Use:   "firewall [args]",
		Short: "add firewall exceptions for enabled tcp listeners, ex.: " + strings.Join(listenerNames, ", "),
		Run:   installFirewall,
	})
}

func installPkg(_ *cobra.Command, args []string) {
	agentFlags.Mode = snclient.ModeOneShot
	snc := snclient.NewAgent(agentFlags)

	installConfig := parseInstallerArgs(args)
	snc.Log.Infof("starting installer: %#v", installConfig)

	// merge tmp_installer.ini into snclient.ini
	mergeIniFile(snc, installConfig)

	// reload config
	_, err := snc.Init()
	if err != nil {
		snc.Log.Errorf("failed to reload config: %s", err.Error())
	}

	switch hasService(WINSERVICE) {
	case false:
		err = installService(WINSERVICE)
		if err != nil {
			snc.Log.Errorf("failed to install service: %s", err.Error())
		}

		err = startService(WINSERVICE)
		if err != nil {
			snc.Log.Errorf("failed to start service: %s", err.Error())
		}
	case true:
		if !serviceEnabled(WINSERVICE) {
			break
		}
		err = restartService(WINSERVICE)
		if err != nil {
			snc.Log.Errorf("failed to (re)start service: %s", err.Error())
		}
	}

	err = addFireWallRule(snc)
	if err != nil {
		snc.Log.Errorf("failed to add firewall: %s", err.Error())
	}
	snc.Log.Infof("installer finished successfully")
	snc.CleanExit(0)
}

func installPre(_ *cobra.Command, args []string) {
	agentFlags.Mode = snclient.ModeOneShot
	snc := snclient.NewAgent(agentFlags)

	installConfig := parseInstallerArgs(args)
	snc.Log.Infof("starting pre script: %#v", installConfig)

	if hasService(WINSERVICE) {
		err := stopService(WINSERVICE)
		if err != nil {
			snc.Log.Infof("failed to stop service: %s", err.Error())
		}
	}
	snc.Log.Infof("pre script finished successfully")
	snc.CleanExit(0)
}

func installFirewall(_ *cobra.Command, _ []string) {
	agentFlags.Mode = snclient.ModeOneShot
	setInteractiveStdoutLogger()
	snc := snclient.NewAgent(agentFlags)

	err := addFireWallRule(snc)
	if err != nil {
		snc.Log.Errorf("failed to add firewall: %s", err.Error())
	}
	snc.Log.Infof("firewall setup ready")
	snc.CleanExit(0)
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
		return fmt.Errorf("mgr.Connect: %s", err.Error())
	}
	defer func() { _ = svcMgr.Disconnect() }()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return fmt.Errorf("svcMgr.OpenService: %s", err.Error())
	}

	err = windows.DeleteService(service.Handle)
	if err != nil {
		return fmt.Errorf("windows.DeleteService: %s", err.Error())
	}

	return nil
}

func hasService(name string) bool {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer func() { _ = svcMgr.Disconnect() }()

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
	defer func() { _ = svcMgr.Disconnect() }()

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
		return fmt.Errorf("mgr.Connect: %s", err.Error())
	}
	defer func() { _ = svcMgr.Disconnect() }()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return fmt.Errorf("svcMgr.OpenService: %s", err.Error())
	}
	defer service.Close()

	state, err := service.Query()
	if err != nil {
		return fmt.Errorf("service.Query: %s", err.Error())
	}
	if state.State != svc.Stopped {
		state, err = service.Control(svc.Stop)
		if err != nil {
			return fmt.Errorf("service.Control: %s", err.Error())
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
			return fmt.Errorf("service.Query: %s", err.Error())
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
		return fmt.Errorf("mgr.Connect: %s", err.Error())
	}
	defer func() { _ = svcMgr.Disconnect() }()

	service, err := svcMgr.OpenService(name)
	if err != nil {
		return fmt.Errorf("svcMgr.OpenService: %s", err.Error())
	}
	defer service.Close()

	err = service.Start()
	if err != nil {
		return fmt.Errorf("service.Start: %s", err.Error())
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

func installService(name string) error {
	if hasService(name) {
		return nil
	}
	svcMgr, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("mgr.Connect: %s", err.Error())
	}
	defer func() { _ = svcMgr.Disconnect() }()

	_, _, execPath, err := utils.GetExecutablePath()
	if err != nil {
		return fmt.Errorf("utils.GetExecutablePath: %s", err.Error())
	}
	_, err = svcMgr.CreateService(
		name,
		execPath,
		mgr.Config{
			StartType:        windows.SERVICE_AUTO_START,
			DelayedAutoStart: true,
			Description:      "SNClient+ (Secure Naemon Client) is a general purpose monitoring agent.",
			SidType:          windows.SERVICE_SID_TYPE_UNRESTRICTED,
		},
		"winservice",
	)
	if err != nil {
		return fmt.Errorf("windows.CreateService: %s", err.Error())
	}

	return nil
}

func mergeIniFile(snc *snclient.Agent, installConfig map[string]string) {
	installDir, ok := installConfig["INSTALLDIR"]
	if !ok {
		snc.Log.Errorf("no install dir found in arguments: %#v", installConfig)

		return
	}

	tmpFile := filepath.Join(installDir, "tmp_installer.ini")
	tmpConfig := snclient.NewConfig(false)
	err := tmpConfig.ParseINIFile(tmpFile, snc)
	if err != nil {
		snc.Log.Errorf("failed to parse %s: %s", tmpFile, err.Error())
	}

	targetFile := filepath.Join(installDir, "snclient.ini")
	targetConfig := snclient.NewConfig(false)
	err = targetConfig.ParseINIFile(targetFile, snc)
	if err != nil {
		snc.Log.Errorf("failed to parse %s: %s", targetFile, err.Error())
	}

	for name, section := range tmpConfig.SectionsByPrefix("/") {
		targetSection := targetConfig.Section(name)
		handleMergeSection(snc, section, targetSection)
	}

	err = targetConfig.WriteINI(targetFile)
	if err != nil {
		snc.Log.Errorf("failed to write %s: %s", targetFile, err.Error())
	}

	err = os.Remove(tmpFile)
	if err != nil {
		snc.Log.Errorf("failed to remove %s: %s", tmpFile, err.Error())
	}
}

func handleMergeSection(snc *snclient.Agent, section, targetSection *snclient.ConfigSection) {
	for _, key := range section.Keys() {
		switch key {
		case "password":
			val, _ := section.GetString(key)
			if val == snclient.DefaultPassword {
				continue
			}

			if val != "" {
				val = toPassword(val)
			}
			targetSection.Insert(key, val)
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

func addFireWallRule(snc *snclient.Agent) error {
	removeFireWallRules(snc)
	snc.Log.Debugf("adding firewall rule '%s'", FIREWALLPREFIX)
	_, _, execPath, err := utils.GetExecutablePath()
	if err != nil {
		return fmt.Errorf("could not detect path to executable: %s", err.Error())
	}
	_ = os.Chdir("C:\\") // avoid: exec: "netsh": cannot run executable found relative to current directory
	cmdLine := []string{
		"advfirewall", "firewall", "add", "rule",
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"profile=any",
		"description=SNClient+ Remote Access",
		fmt.Sprintf("program=%s", execPath),
		fmt.Sprintf("name=%s", FIREWALLPREFIX),
	}
	cmd := exec.Command("netsh", cmdLine...)

	snc.Log.Debugf("adding firewall: netsh %s", strings.Join(cmdLine, " "))

	output, err := cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("failed to create firewall exception: %s (%s)", err.Error(), output)
	}

	snc.Log.Debugf("added firewall: %s", output)

	return nil
}

func removeFireWallRules(snc *snclient.Agent) {
	// previously we added firewall rules for each listen port, so remove them again here
	for _, name := range listenerNames {
		err := removeFireWallRule(snc, name)
		if err != nil {
			snc.Log.Debugf("removeFireWallRule: %s%s: %s", FIREWALLPREFIX, name, err.Error())
		}
	}

	// current firewall rule has no port and uses program only
	err := removeFireWallRule(snc, "")
	if err != nil {
		snc.Log.Debugf("removeFireWallRule: %s: %s", FIREWALLPREFIX, err.Error())
	}
}

func removeFireWallRule(snc *snclient.Agent, name string) error {
	snc.Log.Debugf("removing firewall rule '%s%s'", FIREWALLPREFIX, name)
	_ = os.Chdir("C:\\") // avoid: exec: "netsh": cannot run executable found relative to current directory

	cmdLine := []string{
		"advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s%s", FIREWALLPREFIX, name),
	}
	snc.Log.Debugf("removing firewall: netsh %s", strings.Join(cmdLine, " "))

	cmd := exec.Command("netsh", cmdLine...)

	output, err := cmd.CombinedOutput()
	output = bytes.TrimSpace(output)
	if err != nil {
		return fmt.Errorf("failed to remove firewall exception: %s (%s)", err.Error(), output)
	}

	snc.Log.Debugf("removed firewall: %s", output)

	return nil
}
