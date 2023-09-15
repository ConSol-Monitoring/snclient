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

func makeCmd(ctx context.Context, command, workingDir string) (*exec.Cmd, error) {
	if strings.Contains(command, "LASTEXITCODE") || strings.Contains(command, "lastexitcode") {
		// This is a hack. Without it, neither syscallCommandLineToArgv nor Tokenize will
		// properly parse ...check_sometjing.ps1 "para meter"; exit($LASTEXITCODE)...
		// Result will be [..., `"para meter;"`, ....
		command = strings.ReplaceAll(command, "; exit", " ; exit")
		command = strings.ReplaceAll(command, ";exit", " ; exit")
	}
	cmdList, _ := syscallCommandLineToArgv(command)
	var err error
	cmdList, err = utils.TrimQuotesAll(cmdList)
	if err != nil {
		return nil, fmt.Errorf("trimming arguments: %s", err.Error())
	}
	cmdName := cmdList[0]
	if len(cmdList) == 1 {
		return makeCmdNoParams(ctx, cmdName)
	}
	cmdArgs := cmdList[1:]
	if isBatchFile(cmdName) {
		cmdName = strings.ReplaceAll(cmdName, "__SNCLIENT_BLANK__", "^ ")
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe" // Will be expanded by exec.LookPath in exec.Command
		}
		for i, ca := range cmdArgs {
			cmdArgs[i] = syscall.EscapeArg(ca)
		}
		cmd := exec.CommandContext(ctx, shell, "")
		cmd.Dir = workingDir
		cmd.Args = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
			CmdLine:    fmt.Sprintf(`%s /c %s %s`, shell, cmdName, strings.Join(cmdArgs, " ")),
		}

		return cmd, nil
	}
	if isPsFile(cmdName) {
		for i, ca := range cmdArgs {
			cmdArgs[i] = `'` + ca + `'`
		}
		cmdName = strings.ReplaceAll(cmdName, "__SNCLIENT_BLANK__", " ")
		cmd := exec.CommandContext(ctx, "powershell")
		cmd.Dir = workingDir
		cmd.Args = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
			CmdLine: fmt.Sprintf(`powershell -WindowStyle hidden -NoLogo -NonInteractive -Command ". '%s' %s; exit($LASTEXITCODE)"`,
				cmdName, strings.Join(cmdArgs, " ")),
		}

		return cmd, nil
	}
	cmdName = strings.ReplaceAll(cmdName, "__SNCLIENT_BLANK__", " ")
	for i, ca := range cmdArgs {
		cmdArgs[i] = strings.ReplaceAll(ca, "__SNCLIENT_BLANK__", " ")
	}
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Dir = workingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	return cmd, nil
}

func makeCmdNoParams(ctx context.Context, cmdName string) (*exec.Cmd, error) {
	cmdName = strings.ReplaceAll(cmdName, "__SNCLIENT_BLANK__", " ")
	cmd := exec.CommandContext(ctx, cmdName) // exe and bat even with space in the path can run like this
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	if isBatchFile(cmdName) || !isPsFile(cmdName) {
		return cmd, nil
	}
	cmd = exec.CommandContext(ctx, "powershell")
	cmd.Args = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    fmt.Sprintf(`powershell -WindowStyle hidden -NoLogo -NonInteractive -Command ". '%s'; exit($LASTEXITCODE)"`, cmdName),
	}

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
	argv, err := syscall.CommandLineToArgv(&syscall.StringToUTF16(cmd)[0], &argc) //nolint:staticcheck // copied from a test in the golang repo
	if err != nil {
		return nil, fmt.Errorf("syscall.CommandLineToArgv: %s", err.Error())
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(argv)))) //nolint:errcheck // copied from golang repo

	var args []string //nolint:prealloc // copied from golang repo
	for _, v := range (*argv)[:argc] {
		args = append(args, syscall.UTF16ToString((*v)[:]))
	}

	return args, nil
}

func QuotePathWithSpaces(path string) string {
	components := strings.Split(path, `\`)
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
