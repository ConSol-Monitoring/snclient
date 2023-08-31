package snclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

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

// makeCmd handles the case where the program is a Windows batch os ps1 file
// and the implication it has on argument quoting.
func makeCmd(ctx context.Context, command string) (*exec.Cmd, error) {
	cmdList := utils.Tokenize(command)
	var err error
	cmdList, err = utils.TrimQuotesAll(cmdList)
	if err != nil {
		return nil, err
		//        return nil, errors.Wrap(err, "failed to trim quotes from cmdList")
	}

	cmdName := cmdList[0]
	if len(cmdList) == 1 {
		cmdName = strings.ReplaceAll(cmdList[0], "__BLANK__", " ")
		// binaries and bat can be run in a simple way
		cmd := exec.CommandContext(ctx, cmdName)
		if isBatchFile(cmdName) {
			// calling a bat file without argumentes must be done without escaping/quoting and without cmd.exe
			return cmd, nil
		}
		if !isPsFile(cmdName) {
			// exe files as well
			return cmd, nil
		}
		//  powershell requires wrapping
		shell := "powershell"
		cmd = exec.CommandContext(ctx, shell)
		cmd.Args = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine:    fmt.Sprintf(`%s -NoProfile -NoLogo "%q"`, syscall.EscapeArg(cmd.Path), cmdName),
			HideWindow: true,
		}

		return cmd, nil
	}

	cmdArgs := cmdList[1:]

	if isBatchFile(cmdName) {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe" // Will be expanded by exec.LookPath in exec.Command
		}
		cmd := exec.CommandContext(ctx, shell, "")
		for i, ca := range cmdArgs {
			cmdArgs[i] = syscall.EscapeArg(ca)
		}
		cmdName = strings.ReplaceAll(cmdName, "__BLANK__", "^ ")
		cmdLine := fmt.Sprintf(`%s /c %s %s`, shell, cmdName, strings.Join(cmdArgs, " "))
		cmd.Args = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine:    cmdLine,
			HideWindow: true,
		}
		return cmd, nil
	}
	if isPsFile(cmdName) {
		argsEscaped := make([]string, len(cmdArgs)+1)
		argsEscaped[0] = syscall.EscapeArg(cmdName)
		for i, a := range cmdArgs {
			//argsEscaped[i+1] = syscall.EscapeArg(a)
			argsEscaped[i+1] = a
		}

		shell := "powershell"
		cmd := exec.CommandContext(ctx, shell)
		cmd.Args = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine:    fmt.Sprintf(`%s -NoProfile -NoLogo "%q"`, syscall.EscapeArg(cmd.Path), strings.Join(argsEscaped, " ")),
			HideWindow: true,
		}
	}
	cmdName = strings.ReplaceAll(cmdName, "__BLANK__", " ")
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	return cmd, nil
}

func isBatchFile(path string) bool {
	ext := filepath.Ext(path)

	return strings.EqualFold(ext, ".bat") || strings.EqualFold(ext, ".cmd")
}

func isPsFile(path string) bool {
	ext := filepath.Ext(path)

	return strings.EqualFold(ext, ".ps1")
}

func syscallCommandLineToArgv(cmd string) ([]string, error) {
	var argc int32
	argv, err := syscall.CommandLineToArgv(&syscall.StringToUTF16(cmd)[0], &argc)
	if err != nil {
		return nil, err
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(argv))))

	var args []string
	for _, v := range (*argv)[:argc] {
		args = append(args, syscall.UTF16ToString((*v)[:]))
	}
	return args, nil
}

func QuotePathWithSpaces(path string) string {
	components := strings.Split(path, `/`)
	quotedComponents := make([]string, len(components))

	for i, component := range components {
		if strings.Contains(component, " ") {
			quotedComponents[i] = `"` + component + `"`
		} else {
			quotedComponents[i] = component
		}
	}

	return strings.Join(quotedComponents, `/`)
}

func EscapePathWithSpaces(path string) string {
	components := strings.Split(path, `/`)
	quotedComponents := make([]string, len(components))

	for i, component := range components {
		if strings.Contains(component, " ") {
			quotedComponents[i] = strings.ReplaceAll(component, " ", "^ ")
		} else {
			quotedComponents[i] = component
		}
	}

	return strings.Join(quotedComponents, `/`)
}
