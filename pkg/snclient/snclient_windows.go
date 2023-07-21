package snclient

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pkg/utils"

	"golang.org/x/sys/windows/svc"
)

type winService struct {
	snc *Agent
}

func (m *winService) Execute(_ []string, changeReq <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	go m.snc.Run()

	keepListening := true
	for keepListening {
		chg := <-changeReq
		switch chg.Cmd {
		case svc.Interrogate:
			changes <- chg.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			changes <- chg.CurrentStatus
		case svc.Stop, svc.Shutdown:
			log.Infof("got windows service stop request")
			m.snc.stop()
			keepListening = false
		case svc.Pause,
			svc.Continue,
			svc.ParamChange,
			svc.NetBindAdd,
			svc.NetBindRemove,
			svc.NetBindEnable,
			svc.NetBindDisable,
			svc.DeviceEvent,
			svc.HardwareProfileChange,
			svc.PowerEvent,
			svc.SessionChange,
			svc.PreShutdown:
			// ignored
		default:
			log.Errorf("unexpected control request #%d", chg)
		}
	}
	changes <- svc.Status{State: svc.StopPending}

	return ssec, errno
}

func (snc *Agent) RunAsWinService() {
	const svcName = "snclient"
	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %s", err.Error())
	}
	if !inService {
		log.Fatalf("--daemon mode cannot run interactively on windows")
	}

	err = svc.Run(svcName, &winService{
		snc: snc,
	})
	if err != nil {
		log.Errorf("windows service %s failed: %v", svcName, err)

		return
	}
}

func isInteractive() bool {
	inService, _ := svc.IsWindowsService()

	return !inService
}

func setupUsrSignalChannel(_ chan os.Signal) {
}

func mainSignalHandler(sig os.Signal, _ *Agent) MainStateType {
	switch sig {
	case syscall.SIGTERM:
		log.Infof("got sigterm, quiting gracefully")

		return ShutdownGraceFully
	case os.Interrupt, syscall.SIGINT:
		log.Infof("got sigint, quitting")
		removeTmpExeFiles()

		return Shutdown
	case syscall.SIGHUP:
		log.Infof("got sighup, reloading configuration...")

		return Reload
	default:
		log.Warnf("Signal not handled: %v", sig)
	}

	return Resume
}

func (snc *Agent) finishUpdate(_, mode string) {
	if mode != "winservice" {
		return
	}
	// start service again
	cmd := exec.Command("net", "start", "snclient")
	output, err := cmd.CombinedOutput()
	log.Tracef("[update] net start snclient: %s", strings.TrimSpace(string(output)))
	if err != nil {
		log.Debugf("net start snclient failed: %s", err.Error())
	}
	os.Exit(ExitCodeOK)
}

func (snc *Agent) StartRestartWatcher() {
	getCmd := func() exec.Cmd {
		// need to copy binary, inplace overwrite does not seem to work
		tmpFile := fmt.Sprintf("%s.tmp.%d%s", GlobalMacros["exe-full"], time.Now().UnixMicro(), GlobalMacros["file-ext"])
		LogError(utils.CopyFile(GlobalMacros["exe-full"], tmpFile))
		args := []string{tmpFile}
		for _, a := range os.Args[1:] {
			if a != "watch" && a != "dev" {
				args = append(args, a)
			}
		}
		cmd := exec.Cmd{
			Path:   tmpFile,
			Args:   args,
			Env:    os.Environ(),
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}

		return cmd
	}
	cmd := getCmd()
	LogError(cmd.Start())

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interrupt
		time.Sleep(500 * time.Millisecond)
		removeTmpExeFiles()
		os.Exit(0)
	}()

	snc.running.Store(true)
	snc.restartWatcherCb(func() {
		LogDebug(cmd.Process.Kill())
		_ = cmd.Wait()
		if strings.Contains(cmd.Path, ".tmp.") {
			os.Remove(cmd.Path)
		}
		removeTmpExeFiles()

		cmd = getCmd()
		LogError(cmd.Start())
	})
	os.Exit(ExitCodeOK)
}

func removeTmpExeFiles() {
	files, err := filepath.Glob("snclient.*.tmp.*.exe")
	if err != nil {
		log.Debugf("tmp files remove failed: %s", err.Error())
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			log.Debugf("tmp files remove failed %s: %s", f, err.Error())
		}
	}
}

func processTimeoutKill(process *os.Process) {
	LogDebug(process.Signal(syscall.SIGKILL))
}

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
