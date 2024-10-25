package snclient

import (
	"github.com/consol-monitoring/check_nsc_web/pkg/checknscweb"
)

func init() {
	AvailableChecks["check_nsc_web"] = CheckEntry{"check_nsc_web", NewCheckNSCWeb}
}

func NewCheckNSCWeb() CheckHandler {
	return &CheckBuiltin{
		name: "check_nsc_web",
		description: `Runs check_nsc_web to perform checks on other snclient agents.
It basically wraps the plugin from https://github.com/ConSol-Monitoring/check_nsc_web`,
		check:    checknscweb.Check,
		docTitle: `check_nsc_web`,
		usage:    `check_nsc_web [<options>]`,
		exampleDefault: `
    check_nsc_web -p ... -u https://localhost:8443
    OK - REST API reachable on https://localhost:8443

Check specific plugin:

    check_nsc_web -p ... -u https://localhost:8443 -c check_process process=snclient.exe
    OK - all 1 processes are ok. | ...
`,
		exampleArgs: `'-H' 'omd.consol.de' '--uri=/docs' '-S'`,
	}
}
