package snclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/sni/shelltoken"
)

type CheckAlias struct {
	noCopy  noCopy
	command string
	args    []string // arguments supplied by the alias itself
	config  *ConfigSection
}

func (a *CheckAlias) Build() *CheckData {
	return &CheckData{
		argsPassthrough: true,
	}
}

func (a *CheckAlias) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (res *CheckResult, err error) {
	userArgs := check.rawArgs
	var statusResult *CheckResult
	switch {
	case !checkAllowArguments(a.config, userArgs):
		statusResult = &CheckResult{
			State:  CheckExitUnknown,
			Output: "Exception processing request: Request contained arguments (check the allow arguments option).",
		}
	case !checkNastyCharacters(a.config, "", userArgs):
		statusResult = &CheckResult{
			State:  CheckExitUnknown,
			Output: "Exception processing request: Request contained illegal characters (check the allow nasty characters option).",
		}
	default:
		cmdArgs := a.args
		argStr := strings.Join(a.args, " ")
		if strings.Contains(argStr, "$ARG") {
			log.Debugf("command before macros expanded: %s %s", a.command, argStr)
			macros := map[string]string{
				"ARGS": strings.Join(userArgs, " "),
			}
			for i := range userArgs {
				macros[fmt.Sprintf("ARG%d", i+1)] = check.rawArgs[i]
			}
			fillEmptyArgMacros(macros)

			replacedStr := ReplaceRuntimeMacros(strings.Join(a.args, " "), check.timezone, macros)
			cmdArgs, err = shelltoken.SplitQuotes(replacedStr, shelltoken.Whitespace)
			if err != nil {
				return nil, fmt.Errorf("error parsing command: %s", err.Error())
			}

			log.Debugf("command after macros expanded: %s %s", a.command, replacedStr)
		}
		statusResult, _ = snc.runCheck(ctx, a.command, cmdArgs)
	}

	statusResult.ParsePerformanceDataFromOutputCond(a.command, a.config)

	return statusResult, nil
}
