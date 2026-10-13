package snclient

import (
	"context"
	"io"

	"github.com/consol-monitoring/check_snclient/pkg/checksnclient"
)

func init() {
	AvailableChecks["check_nsc_web"] = CheckEntry{"check_nsc_web", NewCheckSNClient}
	AvailableChecks["check_snclient"] = CheckEntry{"check_snclient", NewCheckSNClient}
}

func NewCheckSNClient() CheckHandler {
	return &CheckBuiltin{
		name: "check_snclient",
		description: `Runs check_snclient (formerly check_nsc_web) to perform checks on other snclient agents.
It basically wraps the plugin from https://github.com/ConSol-Monitoring/check_snclient`,
		check:    checkSNClientWrapper,
		docTitle: `check_snclient`,
		usage:    `check_snclient [<options>]`,
		exampleDefault: `
    check_snclient -p ... -u https://localhost:8443
    OK - REST API reachable on https://localhost:8443

Check specific plugin:

    check_snclient -p ... -u https://localhost:8443 -c check_process process=snclient.exe
    OK - all 1 processes are ok. | ...
`,
		exampleArgs: `'-H' 'omd.consol.de' '--uri=/docs' '-S'`,
	}
}

func checkSNClientWrapper(ctx context.Context, output io.Writer, osArgs []string) int {
	return checksnclient.Check(ctx, output, osArgs, nil)
}
