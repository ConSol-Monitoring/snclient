package snclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"pkg/utils"
)

type CheckWrap struct {
	noCopy        noCopy
	snc           *Agent
	commandString string
	config        *ConfigSection
	wrapped       bool
}

// CheckWrap wraps existing scripts created by the ExternalScriptsHandler
func (l *CheckWrap) Check(snc *Agent, args []string) (*CheckResult, error) {
	l.snc = snc

	cmdToken := utils.Tokenize(l.commandString)
	macros := map[string]string{
		"SCRIPT": cmdToken[0],
		"ARGS":   strings.Join(args, " "),
	}
	for i := range args {
		macros[fmt.Sprintf("ARG%d", i+1)] = args[i]
	}
	command := ReplaceRuntimeMacros(l.commandString, macros)

	// set default timeout
	timeoutSeconds, ok, err := l.config.GetInt("timeout")
	if err != nil || !ok {
		timeoutSeconds = 60
	}

	stdout, stderr, exitCode, procState, err := l.snc.runExternalCommand(command, timeoutSeconds)
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
	if stderr != "" {
		stdout += "\n[" + stderr + "]"
	}
	fixReturnCodes(&stdout, &exitCode, procState)

	return &CheckResult{
		State:  exitCode,
		Output: stdout,
	}, nil
}
