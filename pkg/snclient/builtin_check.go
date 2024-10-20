package snclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

type CheckBuiltin struct {
	name        string
	description string
	check       func(context.Context, io.Writer, []string) int
}

func (l *CheckBuiltin) Build() *CheckData {
	return &CheckData{
		name:            l.name,
		description:     l.description,
		argsPassthrough: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
	}
}

func (l *CheckBuiltin) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	val, _, _ := snc.config.Section("/modules").GetBool("CheckBuiltinPlugins")
	if !val {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "You need to enable CheckBuiltinPlugins in the [/modules] section in order to use this command.",
		}, nil
	}

	val, _, _ = snc.config.Section("/settings/builtin plugins/" + l.name).GetBool("disabled")
	if val {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: fmt.Sprintf("Builtin check plugin %s is disabled in [/settings/builtin plugins/%s].", l.name, l.name),
		}, nil
	}

	output := bytes.NewBuffer(nil)
	rc := l.check(ctx, output, check.rawArgs)
	check.result.Output = output.String()
	check.result.State = int64(rc)

	return check.Finalize()
}

func (l *CheckBuiltin) Help(ctx context.Context, snc *Agent, check *CheckData, _ ShowHelp) (help string) {
	check.rawArgs = []string{"--help"}
	res, _ := l.Check(ctx, snc, check, []Argument{})

	help = string(res.BuildPluginOutput())

	help = strings.TrimSpace(help)

	return help
}
