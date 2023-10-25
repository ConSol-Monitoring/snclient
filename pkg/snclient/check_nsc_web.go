package snclient

import (
	"bytes"
	"context"

	"github.com/consol-monitoring/check_nsc_web/pkg/checknscweb"
)

func init() {
	AvailableChecks["check_nsc_web"] = CheckEntry{"check_nsc_web", new(CheckNSCWeb)}
}

type CheckNSCWeb struct{}

func (l *CheckNSCWeb) Build() *CheckData {
	return &CheckData{
		name:            "check_nsc_web",
		description:     "Runs check_nsc_web to perform checks on other snclient agents.",
		argsPassthrough: true,
		result: &CheckResult{
			State: CheckExitOK,
		},
	}
}

func (l *CheckNSCWeb) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	val, _, _ := snc.Config.Section("/modules").GetBool("CheckBuiltinPlugins")
	if !val {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "You need to enable CheckBuiltinPlugins in the [/modules] section in order to use this command.",
		}, nil
	}

	val, _, _ = snc.Config.Section("/settings/builtin plugins/check_nsc_web").GetBool("disabled")
	if val {
		return &CheckResult{
			State:  CheckExitUnknown,
			Output: "Builtin check plugin check_nsc_web is disabled in [/settings/builtin plugins/check_nsc_web].",
		}, nil
	}

	output := bytes.NewBuffer(nil)
	rc := checknscweb.Check(ctx, output, check.rawArgs)
	check.result.Output = output.String()
	check.result.State = int64(rc)

	return check.Finalize()
}
