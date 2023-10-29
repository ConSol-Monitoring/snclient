package snclient

import (
	"github.com/consol-monitoring/check_nsc_web/pkg/checknscweb"
)

func init() {
	AvailableChecks["check_nsc_web"] = CheckEntry{"check_nsc_web", NewCheckNSCWeb()}
}

func NewCheckNSCWeb() *CheckBuiltin {
	return &CheckBuiltin{
		name:        "check_nsc_web",
		description: "Runs check_nsc_web to perform checks on other snclient agents.",
		check:       checknscweb.Check,
	}
}
