package snclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/utils"
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
	// if snclient is started with verbose arguments, pass them to internal check as well
	args = PrependLogLevelArgs(args, snc.Flags)

	ctx = utils.ContextWithLogger(ctx, log)

	log.Tracef("calling internal check: %s", l.name)
	output := bytes.NewBuffer(nil)
	rc := l.check(ctx, output, args)
	check.result.Output = output.String()
	check.result.State = int64(rc)

	log.Tracef("internal check: %s returned rc: %d", l.name, rc)
	log.Tracef("output: %s", utils.Shorten(check.result.Output, 30, "...")) //nolint:mnd // magic number only used once here, no need to make it a constant

	// reset conditions, they are not used for builtin checks but might interfere with other checks
	check.warnThreshold = nil
	check.critThreshold = nil
	check.okThreshold = nil
	check.filter = nil

	return check.Finalize()
}

func (l *CheckBuiltin) Help(ctx context.Context, snc *Agent, check *CheckData, format ShowHelp) (out string) {
	// use fixed size when printing help pages
	os.Setenv("COLS", "120")

	check.rawArgs = []string{"--help"}
	res, _ := l.Check(ctx, snc, check, []Argument{})

	out = check.helpHeader(format, true)

	usage := res.BuildOutputString()
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
