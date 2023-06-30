package snclient

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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
		stdout, stderr, exitCode, procState, err := l.snc.runExternalCommand(formattedCommand, timeoutSeconds)
		if err != nil {
			exitCode = CheckExitUnknown
			switch {
			case procState == nil:
				stdout = setProcessErrorResult(err)
			case errors.Is(err, context.DeadlineExceeded):
				stdout = fmt.Sprintf("UKNOWN: script run into timeout after %ds\n%s%s", timeoutSeconds, stdout, stderr)
			default:
				stdout = fmt.Sprintf("UKNOWN: script error %s\n%s%s", err.Error(), stdout, stderr)
			}
		}
		fixPluginOutput(&stdout, &stderr)
		if stderr != "" {
			stdout += "\n[" + stderr + "]"
		}
		fixReturnCodes(&stdout, &exitCode, procState)
		output = stdout
		state = exitCode
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}
