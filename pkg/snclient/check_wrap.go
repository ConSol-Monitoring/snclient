package snclient

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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

	var command string
	if l.wrapped {
		ext := strings.TrimPrefix(filepath.Ext(cmdToken[0]), ".")
		wrapping, ok := snc.Config.Section("/settings/external scripts/wrappings").GetString(ext)
		if !ok {
			return nil, fmt.Errorf("no wrapping found for extension: %s", ext)
		}
		command = ReplaceRuntimeMacros(wrapping, macros)
	} else {
		command = ReplaceRuntimeMacros(l.commandString, macros)
	}

	if strings.Contains(command, "script root") {
		scriptRoot, ok := snc.Config.Section("/settings/external scripts").GetString("script root")
		if ok {
			macrosRoot := map[string]string{
				"script root": scriptRoot,
			}
			command = ReplaceMacros(command, macrosRoot)
		}
	}

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
