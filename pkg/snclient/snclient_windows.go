package snclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pkg/utils"

	"github.com/sni/shelltoken"
	"golang.org/x/sys/windows"
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

func IsInteractive() bool {
	if inService, _ := svc.IsWindowsService(); inService {
		return false
	}

	if _, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE); err != nil {
		return false
	}

	return true
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

func (snc *Agent) finishUpdate(binPath, mode string) {
	if mode == "update" {
		cmd := exec.Command(binPath, "update", "apply")
		cmd.Env = os.Environ()
		_ = cmd.Start()

		return
	}
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

func (snc *Agent) makeCmd(ctx context.Context, command string) (*exec.Cmd, error) {
	cmdName, cmdArgs, hasShellCode, err := snc.shellParse(command)
	if err != nil {
		return nil, err
	}

	shell := shell()
	_, lookupErr := exec.LookPath(cmdName)

	// add scripts path to PATH env
	scriptsPath, _ := snc.Config.Section("/paths").GetString("scripts")
	env := append(os.Environ(), "PATH="+scriptsPath+";"+os.Getenv("PATH"))

	switch {
	// powershell command
	case strings.HasPrefix(command, "& "):
		cmd := execCommandContext(ctx, "powershell", env)
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(
			`powershell -WindowStyle hidden -NoLogo -NonInteractive -Command %s; exit($LASTEXITCODE)`,
			command,
		)

		return cmd, nil

	// command does not exist
	case lookupErr != nil:
		return nil, fmt.Errorf("UNKNOWN - Return code of 127 is out of bounds. Make sure the plugin you're trying to run actually exists.\n%s", lookupErr.Error())

	// .bat files
	case isBatchFile(cmdName):
		for i, ca := range cmdArgs {
			cmdArgs[i] = syscall.EscapeArg(ca)
		}
		cmd := execCommandContext(ctx, shell, env, "")
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(
			`%s /c %s %s`,
			shell,
			strings.ReplaceAll(cmdName, " ", "^ "),
			strings.Join(cmdArgs, " "),
		)

		return cmd, nil

	// powershell files
	case isPsFile(cmdName):
		for i, ca := range cmdArgs {
			cmdArgs[i] = `'` + ca + `'`
		}
		cmd := execCommandContext(ctx, "powershell", env)
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(
			`powershell -WindowStyle hidden -NoLogo -NonInteractive -Command ". '%s' %s; exit($LASTEXITCODE)"`,
			cmdName,
			strings.Join(cmdArgs, " "),
		)

		return cmd, nil

	// other command but no shell special characters
	case !hasShellCode:
		cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
		cmd.Env = env

		return cmd, nil

	// nothing else matched, try cmd.exe as last ressort but least replace command name spaces so they work with cmd.exe
	default:
		cmd := execCommandContext(ctx, shell, env)
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(
			`%s /c %s`,
			shell,
			strings.Replace(command, cmdName, syscall.EscapeArg(cmdName), 1))

		return cmd, nil
	}
}

func (snc *Agent) shellParse(command string) (cmdName string, args []string, hasShellCode bool, err error) {
	_, args, err = shelltoken.SplitWindows(command)
	if err != nil {
		if errors.Is(err, &shelltoken.ShellCharactersFoundError{}) {
			hasShellCode = true
		} else {
			return "", nil, false, fmt.Errorf("command parse error: %s", err.Error())
		}
	}

	args = snc.fixPathHoles(args)
	cmdName = args[0]
	args = args[1:]

	return
}

func execCommandContext(ctx context.Context, cmdName string, env []string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Args = nil
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	return cmd
}

// return default shell command
func shell() string {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe" // Will be expanded by exec.LookPath in exec.Command
	}

	return shell
}

func powerShellCmd(ctx context.Context, command string) (cmd *exec.Cmd) {
	cmd = exec.CommandContext(ctx, "powershell")
	cmd.Args = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    fmt.Sprintf(`powershell -WindowStyle hidden -NoLogo -NonInteractive -Command "%s"`, command), //nolint:gocritic // using %q just breaks the command from escaping newlines
	}

	return cmd
}

func isBatchFile(path string) bool {
	ext := filepath.Ext(path)

	return strings.EqualFold(ext, ".bat") || strings.EqualFold(ext, ".cmd")
}

func isPsFile(path string) bool {
	ext := filepath.Ext(path)

	return strings.EqualFold(ext, ".ps1")
}

func setCmdUser(_ *exec.Cmd, _ string) error {
	return fmt.Errorf("droping privileges is not supported on windows")
}

// fix using paths with spaces in its name, parsed into separate pieces
// ex.: C:\Program Files\...
func (snc *Agent) fixPathHoles(cmdAndArgs []string) []string {
	scriptsPath, _ := snc.Config.Section("/paths").GetString("scripts")
	for pieceNo := range cmdAndArgs {
		cmdPath := strings.Join(cmdAndArgs[0:pieceNo+1], " ")
		realPath, err := exec.LookPath(cmdPath)

		if err == nil {
			// can be abs., /usr/bin/echo or %scripts%/check_xy
			// or rel., echo, timeout, anything in $PATH
			fixed := []string{realPath}
			fixed = append(fixed, cmdAndArgs[pieceNo+1:]...)

			return fixed
		}

		if filepath.IsAbs(cmdPath) {
			continue
		}

		// try a relative lookup in %scripts%
		realPath, err = exec.LookPath(filepath.Join(scriptsPath, cmdPath))
		if err == nil {
			fixed := []string{realPath}
			fixed = append(fixed, cmdAndArgs[pieceNo+1:]...)

			return fixed
		}
	}

	return cmdAndArgs
}
