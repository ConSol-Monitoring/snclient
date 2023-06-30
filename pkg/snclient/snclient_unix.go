//go:build !windows

package snclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"pkg/utils"
)

func isInteractive() bool {
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

func (snc *Agent) runExternalCommand(command string, timeout int64) (stdout, stderr string, exitCode int64, proc *os.ProcessState, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)

	// byte buffer for output
	var errbuf bytes.Buffer
	var outbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	// prevent child from receiving signals meant for the agent only
	setSysProcAttr(cmd)

	err = cmd.Start()
	if err != nil && cmd.ProcessState == nil {
		return "", "", ExitCodeUnknown, nil, fmt.Errorf("proc: %s", err.Error())
	}

	// https://github.com/golang/go/issues/18874
	// timeout does not work for child processes and/or if file handles are still open
	go func(proc *os.Process) {
		defer snc.logPanicExit()
		<-ctx.Done() // wait till command runs into timeout or is finished (canceled)
		if proc == nil {
			return
		}
		cmdErr := ctx.Err()
		switch {
		case errors.Is(cmdErr, context.DeadlineExceeded):
			// timeout
			processTimeoutKill(proc)
		case errors.Is(cmdErr, context.Canceled):
			// normal exit
			LogDebug(proc.Kill())
		}
	}(cmd.Process)

	err = cmd.Wait()
	cancel()
	if err != nil && cmd.ProcessState == nil {
		return "", "", ExitCodeUnknown, nil, fmt.Errorf("proc: %s", err.Error())
	}

	state := cmd.ProcessState

	ctxErr := ctx.Err()
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return "", "", ExitCodeUnknown, state, fmt.Errorf("timeout: %s", ctxErr.Error())
	}

	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		exitCode = int64(waitStatus.ExitStatus())
	}

	// extract stdout and stderr
	stdout = string(bytes.TrimSpace((bytes.Trim(outbuf.Bytes(), "\x00"))))
	stderr = string(bytes.TrimSpace((bytes.Trim(errbuf.Bytes(), "\x00"))))

	return stdout, stderr, exitCode, state, nil
}

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
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
