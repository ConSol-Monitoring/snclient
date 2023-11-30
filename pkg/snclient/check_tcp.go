package snclient

import "pkg/check_tcp"

func init() {
	AvailableChecks["check_tcp"] = CheckEntry{"check_tcp", NewCheckTCP}
}

func NewCheckTCP() CheckHandler {
	return &CheckBuiltin{
		name:        "check_tcp",
		description: "Runs check_tcp to perform checks on other snclient agents.",
		check:       check_tcp.Check,
	}
}
