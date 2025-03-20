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

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/sni/shelltoken"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

const svcWaitTimeOut = 10 * time.Second

type winService struct {
	snc *Agent
}

func (m *winService) Execute(_ []string, changeReq <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	// change working directory to shared-path (ex.: C:\Program Files\snclient) so relative paths in scripts will work
	sharedPath, _ := m.snc.config.Section("/paths").GetString("shared-path")
	err := os.Chdir(sharedPath)
	if err != nil {
		log.Fatalf("failed to change working directory to %s: %s", sharedPath, err.Error())
	}

	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	go m.snc.Run()
	if !m.snc.StartWait(svcWaitTimeOut) {
		log.Fatalf("snclient failed to start within %s", svcWaitTimeOut.String())
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	keepListening := true
	for keepListening {
		select {
		case chg := <-changeReq:
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
		case <-ticker.C:
			if !m.snc.IsRunning() {
				log.Debugf("main loop exited, stopping windows service")
				keepListening = false

				break
			}
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
		err := cmd.Start()
		if err != nil {
			log.Errorf("failed to start update apply: %s", err.Error())

			return
		}
		go func() {
			err := cmd.Wait()
			if err != nil {
				log.Errorf("update apply failed: %s", err.Error())
			}
		}()

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
	scriptsPath, _ := snc.config.Section("/paths").GetString("scripts")
	env := append(os.Environ(), "PATH="+scriptsPath+";"+os.Getenv("PATH"))

	switch {
	// powershell command
	case strings.HasPrefix(command, "& "):
		cmd := execCommandContext(ctx, "powershell", env)
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(`%s -command %s; exit($LASTEXITCODE)`, POWERSHELL, command)

		return cmd, nil

	// command does not exist
	case lookupErr != nil:
		cwd, _ := os.Getwd()
		//nolint:stylecheck // error strings must be capitalized here because it ends up like this in the plugin output
		return nil, fmt.Errorf("Return code of 127 is out of bounds. Make sure the plugin you're trying to run actually exists (current working directory: %s).\n%s", cwd, lookupErr.Error())

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
			if strings.ContainsAny(ca, " \t") {
				cmdArgs[i] = `'` + ca + `'`
			}
		}
		cmd := execCommandContext(ctx, "powershell", env)
		cmd.SysProcAttr.CmdLine = fmt.Sprintf(`%s -Command ". '%s' %s; exit($LASTEXITCODE)"`, POWERSHELL, cmdName, strings.Join(cmdArgs, " "))

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
	args, err = shelltoken.SplitQuotes(command, shelltoken.Whitespace, shelltoken.SplitKeepBackslashes|shelltoken.SplitContinueOnShellCharacters)
	if err != nil {
		tst := &shelltoken.ShellCharactersFoundError{}
		if errors.As(err, &tst) {
			hasShellCode = true
			err = nil
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
		CmdLine:    fmt.Sprintf(`%s -Command "%s"`, POWERSHELL, command), //nolint:gocritic // using %q just breaks the command from escaping newlines
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
	scriptsPath, _ := snc.config.Section("/paths").GetString("scripts")
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
