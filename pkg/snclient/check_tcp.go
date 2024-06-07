package snclient

import "github.com/consol-monitoring/snclient/pkg/check_tcp"

func init() {
	AvailableChecks["check_tcp"] = CheckEntry{"check_tcp", NewCheckTCP}
}

func NewCheckTCP() CheckHandler {
	return &CheckBuiltin{
		name:        "check_tcp",
		description: "Runs check_tcp to perform tcp connection checks.",
		check:       check_tcp.Check,
	}
}
