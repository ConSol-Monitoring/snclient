//go:build !windows

package snclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"pkg/convert"
	"pkg/utils"
)

func IsInteractive() bool {
	o, _ := os.Stdout.Stat()
	// check if attached to terminal.
	return (o.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}

func setupUsrSignalChannel(osSignalUsrChannel chan os.Signal) {
	signal.Notify(osSignalUsrChannel, syscall.SIGUSR1)
	signal.Notify(osSignalUsrChannel, syscall.SIGUSR2)
}

func mainSignalHandler(sig os.Signal, snc *Agent) MainStateType {
	switch sig {
	case syscall.SIGTERM:
		log.Infof("got sigterm, quiting gracefully")

		return ShutdownGraceFully
	case os.Interrupt, syscall.SIGINT:
		log.Infof("got sigint, quitting")

		return Shutdown
	case syscall.SIGHUP:
		log.Infof("got sighup, reloading configuration...")

		return Reload
	case syscall.SIGUSR1:
		log.Errorf("requested thread dump via signal %s", sig)
		utils.LogThreadDump(log)

		return Resume
	case syscall.SIGUSR2:
		if snc.flags.ProfileMem == "" {
			log.Errorf("requested memory profile, but flag -memprofile missing")

			return (Resume)
		}

		memFile, err := os.Create(snc.flags.ProfileMem)
		if err != nil {
			log.Errorf("could not create memory profile: %s", err.Error())
		}
		defer memFile.Close()

		runtime.GC()

		if err := pprof.WriteHeapProfile(memFile); err != nil {
			log.Errorf("could not write memory profile: %s", err.Error())
		}

		log.Warnf("memory profile written to: %s", snc.flags.ProfileMem)

		return (Resume)
	default:
		log.Warnf("Signal not handled: %v", sig)
	}

	return Resume
}

func (snc *Agent) finishUpdate(binPath, _ string) {
	log.Tracef("[update] reexec into new file %s %v", binPath, os.Args[1:])
	err := syscall.Exec(binPath, os.Args, os.Environ()) //nolint:gosec // false positive? There should be no tainted input here
	if err != nil {
		log.Errorf("restart failed: %s", err.Error())
	}
	os.Exit(ExitCodeError)
}

func (snc *Agent) StartRestartWatcher() {
	go func() {
		defer snc.logPanicExit()
		binFile := GlobalMacros["exe-full"]
		snc.restartWatcherCb(func() {
			up := &UpdateHandler{snc: snc}
			LogError(up.ApplyRestart(binFile))
		})
	}()
}

func processTimeoutKill(process *os.Process) {
	go func(pid int) {
		// kill the process itself and the hole process group
		LogDebug(syscall.Kill(-pid, syscall.SIGTERM))
		time.Sleep(1 * time.Second)

		LogDebug(syscall.Kill(-pid, syscall.SIGINT))
		time.Sleep(1 * time.Second)

		LogDebug(syscall.Kill(-pid, syscall.SIGKILL))
	}(process.Pid)
}

func makeCmd(ctx context.Context, command string) (*exec.Cmd, error) {
	command = strings.ReplaceAll(command, "__SNCLIENT_BLANK__", "\\ ")
	cmdList := []string{"/bin/sh", "-c", command}
	cmd := exec.CommandContext(ctx, cmdList[0], cmdList[1:]...) // #nosec G204
	// prevent child from receiving signals meant for the agent only
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return cmd, nil
}

func setCmdUser(cmd *exec.Cmd, username string) error {
	usr, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user.lookup: %s: %s", username, err.Error())
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	uid, err := convert.IntE(usr.Uid)
	if err != nil {
		return fmt.Errorf("cannot convert uid to number for user %s (uid:%s): %s", username, usr.Uid, err.Error())
	}

	gid, err := convert.IntE(usr.Gid)
	if err != nil {
		return fmt.Errorf("cannot convert gid to number for user %s (gid:%s): %s", username, usr.Gid, err.Error())
	}

	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}

	return nil
}
