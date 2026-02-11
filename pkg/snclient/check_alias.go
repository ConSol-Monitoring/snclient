package snclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
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
		name:            a.command,
		argsPassthrough: true,
	}
}

func (a *CheckAlias) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (res *CheckResult, err error) {
	cmdArgs := a.args
	argStr := utils.JoinQuoted(a.args)
	if strings.Contains(argStr, "$ARG") {
		log.Debugf("command before macros expanded: %s %s", a.command, argStr)
		macros := map[string]string{
			"ARGS": strings.Join(check.rawArgs, " "),
		}
		for i := range check.rawArgs {
			macros[fmt.Sprintf("ARG%d", i+1)] = check.rawArgs[i]
		}
		fillEmptyArgMacros(macros)

		replacedStr := ReplaceRuntimeMacros(argStr, check.timezone, macros)
		cmdArgs, err = shelltoken.SplitQuotes(replacedStr, shelltoken.Whitespace)
		if err != nil {
			return nil, fmt.Errorf("error parsing command: %s", err.Error())
		}

		log.Debugf("command after macros expanded: %s %s", a.command, replacedStr)
	}
	statusResult, _ := snc.runCheck(ctx, a.command, cmdArgs, check.timeout, nil, true, true)

	statusResult.ParsePerformanceDataFromOutputCond(a.command, a.config)

	return statusResult, nil
}
