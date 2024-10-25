package snclient

import "github.com/consol-monitoring/snclient/pkg/check_dns"

func init() {
	AvailableChecks["check_dns"] = CheckEntry{"check_dns", NewCheckDNS}
}

func NewCheckDNS() CheckHandler {
	return &CheckBuiltin{
		name: "check_dns",
		description: `Runs check_dns to perform nameserver checks.
It basically wraps the plugin from https://github.com/mackerelio/go-check-plugins/tree/master/check-dns`,
		check:    check_dns.Check,
		docTitle: `check_dns`,
		usage:    `check_dns [<options>]`,
		exampleDefault: `
Alert if dns server does not respond:

    check_dns -H labs.consol.de
    OK - labs.consol.de returns 94.185.89.33 (A)

Check for specific type from specific server:

    check_dns -H consol.de -q MX -s 1.1.1.1
    OK - consol.de returns mail.consol.de. (MX)
	`,
		exampleArgs: `'-H' 'omd.consol.de'`,
	}
}
