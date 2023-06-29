package snclient

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type CheckWrap struct {
	noCopy        noCopy
	snc           *Agent
	data          CheckData
	commandString string
	config        *ConfigSection
	wrapped       bool
}

// CheckWrap wraps existing scripts created by the ExternalScriptsHandler
func (l *CheckWrap) Check(snc *Agent, args []string) (*CheckResult, error) {
	l.snc = snc

	var err error
	var state int64
	argList, err := l.data.ParseArgs(args)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	scriptArgs := map[string]string{}
	formattedCommand := l.commandString
	winExecutable := "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"

	for _, arg := range argList {
		switch arg.key {
		case "SCRIPT", "script":
			scriptArgs["SCRIPT"] = arg.value
		case "ARGS", "args":
			scriptArgs["ARGS"] = arg.value
		}
	}

	if _, exists := scriptArgs["ARGS"]; !exists {
		args := []string{}
		for _, arg := range argList {
			args = append(args, arg.key)
		}
		scriptArgs["ARGS"] = strings.Join(args, " ")
	}
	argRe := regexp.MustCompile(`[%$](\w+)[%$]`)
	matches := argRe.FindAllStringSubmatch(formattedCommand, -1)

	for _, match := range matches {
		r := regexp.MustCompile(regexp.QuoteMeta(match[0]))
		formattedCommand = r.ReplaceAllString(formattedCommand, scriptArgs[match[1]])
	}

	timeoutSeconds, ok, err := l.config.GetInt("timeout")
	if err != nil || !ok {
		timeoutSeconds = 60
	}

	var output string
	//nolint:gosec // tainted input is known and unavoidable
	switch runtime.GOOS {
	case "windows":
		log.Debugf("executing command: %s %s", winExecutable, "Set-ExecutionPolicy -Scope Process Unrestricted -Force;"+formattedCommand+"; $LASTEXITCODE")
		scriptOutput, err := exec.Command(winExecutable, "Set-ExecutionPolicy -Scope Process Unrestricted -Force;"+formattedCommand+"; $LASTEXITCODE").CombinedOutput()
		re := regexp.MustCompile(`(\d+)\s*\z`)
		match := re.FindStringSubmatch(string(scriptOutput))
		if len(match) > 0 {
			state, _ = strconv.ParseInt(match[1], 10, 64)
			output = re.ReplaceAllString(string(scriptOutput), "")
		} else {
			state = 3
			output = fmt.Sprintf("Unknown Error in Script: %s", err)
		}
	default:
		output, state = l.runExternalCommand(formattedCommand, timeoutSeconds)
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}

func (l *CheckWrap) runExternalCommand(command string, timeout int64) (output string, exitCode int64) {
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

	err := cmd.Start()
	if err != nil && cmd.ProcessState == nil {
		return setProcessErrorResult(err)
	}

	// https://github.com/golang/go/issues/18874
	// timeout does not work for child processes and/or if file handles are still open
	go func(proc *os.Process) {
		defer l.snc.logPanicExit()
		<-ctx.Done() // wait till command runs into timeout or is finished (canceled)
		if proc == nil {
			return
		}
		switch ctx.Err() {
		case context.DeadlineExceeded:
			// timeout
			processTimeoutKill(proc)
		case context.Canceled:
			// normal exit
			proc.Kill()
		}
	}(cmd.Process)

	err = cmd.Wait()
	cancel()
	if err != nil && cmd.ProcessState == nil {
		return setProcessErrorResult(err)
	}

	state := cmd.ProcessState

	if ctx.Err() == context.DeadlineExceeded {
		output = fmt.Sprintf("UKNOWN: script run into timeout after %ds", timeout)
		exitCode = CheckExitUnknown

		return
	}

	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		exitCode = int64(waitStatus.ExitStatus())
	}

	// extract stdout and stderr
	output = string(bytes.TrimSpace((bytes.Trim(outbuf.Bytes(), "\x00"))))
	errStr := string(bytes.TrimSpace((bytes.Trim(errbuf.Bytes(), "\x00"))))
	if errStr != "" {
		output += "\n[" + errStr + "]"
	}

	fixReturnCodes(&output, &exitCode, state)
	output = strings.Replace(strings.Trim(output, "\r\n"), "\n", `\n`, len(output))

	return output, exitCode
}

func setProcessErrorResult(err error) (output string, exitCode int64) {
	if os.IsNotExist(err) {
		output = fmt.Sprintf("UNKNOWN: Return code of 127 is out of bounds. Make sure the plugin you're trying to run actually exists.")
		exitCode = CheckExitUnknown

		return
	}
	if os.IsPermission(err) {
		output = fmt.Sprintf("UNKNOWN: Return code of 126 is out of bounds. Make sure the plugin you're trying to run is executable.")
		exitCode = CheckExitUnknown

		return
	}
	log.Errorf("system error: %w", err)
	exitCode = CheckExitUnknown
	output = fmt.Sprintf("UNKNOWN: %s", err.Error())

	return
}

func fixReturnCodes(output *string, exitCode *int64, state *os.ProcessState) {
	if *exitCode >= 0 && *exitCode <= 3 {
		return
	}
	if *exitCode == 126 {
		*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds. Make sure the plugin you're trying to run is executable.\n%s", *exitCode, *output)
		*exitCode = 2

		return
	}
	if *exitCode == 127 {
		*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds. Make sure the plugin you're trying to run actually exists.\n%s", *exitCode, *output)
		*exitCode = 2

		return
	}
	if waitStatus, ok := state.Sys().(syscall.WaitStatus); ok {
		if waitStatus.Signaled() {
			*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds. Plugin exited by signal: %s.\n%s", waitStatus.Signal(), waitStatus.Signal(), *output)
			*exitCode = 2

			return
		}
	}
	*output = fmt.Sprintf("CRITICAL: Return code of %d is out of bounds.\n%s", *exitCode, *output)
	*exitCode = 3
}

func processTimeoutKill(p *os.Process) {
	go func(pid int) {
		// kill the process itself and the hole process group
		syscall.Kill(-pid, syscall.SIGTERM)
		time.Sleep(1 * time.Second)

		syscall.Kill(-pid, syscall.SIGINT)
		time.Sleep(1 * time.Second)

		syscall.Kill(-pid, syscall.SIGKILL)
	}(p.Pid)
}
