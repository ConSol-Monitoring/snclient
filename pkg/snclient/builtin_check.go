package snclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type CheckBuiltin struct {
	name           string
	description    string
	check          func(context.Context, io.Writer, []string) int
	usage          string
	exampleDefault string
	exampleArgs    string
	docTitle       string
}

func (l *CheckBuiltin) Build() *CheckData {
	return &CheckData{
		name:            l.name,
		description:     l.description,
		argsPassthrough: true,
		implemented:     ALL,
		result: &CheckResult{
			State: CheckExitOK,
		},
		docTitle:       l.docTitle,
		usage:          l.usage,
		exampleDefault: l.exampleDefault,
		exampleArgs:    l.exampleArgs,
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

	args := []string{}
	args = append(args, check.rawArgs...)
	switch {
	case snc.flags.Verbose >= 3:
		args = append(args, "-vvv")
	case snc.flags.Verbose >= 2:
		args = append(args, "-vv")
	case snc.flags.Verbose >= 1:
		args = append(args, "-v")
	}

	output := bytes.NewBuffer(nil)
	rc := l.check(ctx, output, args)
	check.result.Output = output.String()
	check.result.State = int64(rc)

	return check.Finalize()
}

func (l *CheckBuiltin) Help(ctx context.Context, snc *Agent, check *CheckData, format ShowHelp) (out string) {
	check.rawArgs = []string{"--help"}
	res, _ := l.Check(ctx, snc, check, []Argument{})

	out = check.helpHeader(format, true)

	usage := string(res.BuildPluginOutput())
	usage = regexp.MustCompile(`(?m)^\s+$`).ReplaceAllString(usage, "")
	if format == Markdown {
		out += "## Usage\n\n"
		out += "```"
		out += usage
		out += "```\n"
	} else {
		out += "Usage:\n\n    "
		usage = "    " + strings.Join(strings.Split(usage, "\n"), "\n    ")
		out += usage
	}

	out = strings.TrimSpace(out)

	return out
}
