//go:build !windows

package snclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/utils"
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
		if snc.Flags.ProfileMem == "" {
			log.Errorf("requested memory profile, but flag -memprofile missing")

			return Resume
		}

		memFile, err := os.Create(snc.Flags.ProfileMem)
		if err != nil {
			log.Errorf("could not create memory profile: %s", err.Error())
		}
		defer memFile.Close()

		runtime.GC()

		if err := pprof.WriteHeapProfile(memFile); err != nil {
			log.Errorf("could not write memory profile: %s", err.Error())
		}

		log.Warnf("memory profile written to: %s", snc.Flags.ProfileMem)

		return Resume
	default:
		log.Warnf("Signal not handled: %v", sig)
	}

	return Resume
}

func (snc *Agent) finishUpdate(binPath, mode string) {
	if mode == "update" {
		cmd := exec.CommandContext(context.TODO(), binPath, "update", "apply")
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
	if mode != "daemon" && mode != "server" {
		return
	}

	if err := snc.checkFileOwner(binPath); err != nil {
		log.Debugf("[update] owner check of %s: %s", binPath, err.Error())
		log.Errorf("[update] refusing to exec into %s, owner mismatch", binPath)

		return
	}

	log.Debugf("[update] re-exec into new file %s %#v", binPath, os.Args[1:])
	// prepare capabilities which previously have been removed for all child processes, but in this case are required again
	runtime.LockOSThread()
	LogError(prepareCapsForExec())
	err := syscall.Exec(binPath, os.Args, os.Environ()) //nolint:gosec // false positive? There should be no tainted input here
	if err != nil {
		LogError(clearInheritableCaps()) // in case of an error, clear the inheritable caps again to not leak them to other processes
		runtime.UnlockOSThread()
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
			LogError(up.ApplyRestart(binFile, RestartAlways))
		})
	}()
}

func processTimeoutKill(process *os.Process) {
	go func(pid int) {
		// kill the process itself and the hole process group
		LogDebug(syscall.Kill(-pid, syscall.SIGTERM))
		time.Sleep(1 * time.Second)

		LogTrace(syscall.Kill(-pid, syscall.SIGINT))
		time.Sleep(1 * time.Second)

		LogTrace(syscall.Kill(-pid, syscall.SIGKILL))
	}(process.Pid)
}

func processKill(process *os.Process) {
	go func(pid int) {
		err := syscall.Kill(-pid, syscall.SIGTERM)
		switch {
		case err == nil:
			// process killed successfully, keep going and make sure its gone
		case errors.Is(err, syscall.ESRCH): // process already exited
			return
		}
		time.Sleep(100 * time.Millisecond)
		err = syscall.Kill(-pid, syscall.SIGKILL)
		switch {
		case err == nil:
			// process killed successfully
		case errors.Is(err, syscall.ESRCH): // process already exited
			return
		default:
			log.Errorf("failed to kill process %d: %s", pid, err.Error())
		}
	}(process.Pid)
}

func setCmdUser(cmd *exec.Cmd, username string) error {
	usr, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user.lookup: %s: %s", username, err.Error())
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	uid, err := convert.UInt32E(usr.Uid)
	if err != nil {
		return fmt.Errorf("cannot convert uid to number for user %s (uid:%s): %s", username, usr.Uid, err.Error())
	}

	gid, err := convert.UInt32E(usr.Gid)
	if err != nil {
		return fmt.Errorf("cannot convert gid to number for user %s (gid:%s): %s", username, usr.Gid, err.Error())
	}

	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}

	return nil
}

func (snc *Agent) makeCmd(ctx context.Context, command string) (*exec.Cmd, error) {
	// capabilities are bound to threads, so make sure we are on the same thread when we drop them and execute the command
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := clearInheritableCaps()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command) // #nosec G204
	// prevent child from receiving signals meant for the agent only
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// add scripts path to PATH env
	scriptsPath, _ := snc.config.Section("/paths").GetString("scripts")
	cmd.Env = append(os.Environ(), "PATH="+scriptsPath+":"+os.Getenv("PATH"))

	return cmd, nil
}

func (snc *Agent) checkFileOwner(path string) error {
	uid := os.Geteuid()
	if uid == -1 {
		return fmt.Errorf("cannot determine current user, got user id: %d", uid)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to retrieve file metadata")
	}

	uid32, err := convert.UInt32E(uid)
	if err != nil {
		return fmt.Errorf("cannot convert uid to uint32: %w", err)
	}

	if stat.Uid != uid32 {
		return fmt.Errorf("directory %s is not owned by the current user", path)
	}

	return nil
}

// reap all inherited processes on startup to not accumulate zombies
func reapInheritedChildProcesses() {
	for {
		var status syscall.WaitStatus
		// WNOHANG: return immediately if no child has exited.
		// pid -1: wait for any child process.
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if err != nil {
			// ECHILD means no children exist at all — nothing to reap.
			if errors.Is(err, syscall.ECHILD) {
				return
			}

			// EINTR: interrupted, try again.
			if errors.Is(err, syscall.EINTR) {
				time.Sleep(100 * time.Millisecond)

				continue
			}

			log.Warnf("wait4 error: %s", err.Error())

			return
		}

		// pid == 0: no more exited children to reap right now.
		if pid <= 0 {
			return
		}

		log.Warnf("reaped inherited child pid=%d status=%v", pid, status)
	}
}
