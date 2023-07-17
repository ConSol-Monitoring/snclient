package cmd

import (
	"os"
	"strings"
	"time"

	"pkg/snclient"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	installCmd := &cobra.Command{
		Use:   "install [cmd]",
		Short: "Install windows service and firewall exception",
		Long: `Install is used during msi installation for adding the windows service and a firewall exception.
`,
		Run: func(cmd *cobra.Command, args []string) {
			agentFlags.Mode = snclient.ModeOneShot
			snc := snclient.NewAgent(agentFlags)

			installConfig := parseInstallerArgs(args[0])
			snc.Log.Errorf("****** install: %#v", installConfig)
			if hasService("snclient") {
				snc.Log.Errorf("windows service does already exist")
			} else {
				err := installService(
					"snclient",
					"snclient",
					"SNClient+ (Secure Naemon Client) is a secure general purpose monitoring agent.",
					[]string{"winservice"},
				)
				if err != nil {
					snc.Log.Errorf("failed to install service: %s", err.Error())
				}
			}

			err := restartService("snclient")
			if err != nil {
				snc.Log.Errorf("failed to start service: %s", err.Error())
			}

			os.Exit(0)
		},
	}
	rootCmd.AddCommand(installCmd)
}

func parseInstallerArgs(args string) map[string]string {
	parsed := make(map[string]string, 0)

	for _, a := range strings.Split(args, "; ") {
		val := strings.SplitN(a, "=", 2)
		val[1] = strings.TrimSuffix(val[1], ";")
		parsed[val[0]] = val[1]
	}

	return parsed
}

func installService(name, displayName, description string, args []string) error {
	svcMgr, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer svcMgr.Disconnect()

	service, err := svcMgr.CreateService(name, os.Args[0], mgr.Config{
		DisplayName:      displayName,
		StartType:        mgr.StartAutomatic,
		Description:      description,
		DelayedAutoStart: true,
	}, args...)
	if err != nil {
		return err
	}
	service.Close()
	return nil
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

	// Wait for the service to stop
	for state.State != svc.Stopped {
		time.Sleep(500 * time.Millisecond)
		state, err = service.Query()
		if err != nil {
			return err
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
