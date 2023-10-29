package snclient

import "pkg/check_dns"

func init() {
	AvailableChecks["check_dns"] = CheckEntry{"check_dns", NewCheckDNS()}
}

func NewCheckDNS() *CheckBuiltin {
	return &CheckBuiltin{
		name:        "check_dns",
		description: "Runs check_dns to perform checks on other snclient agents.",
		check:       check_dns.Check,
	}
}
