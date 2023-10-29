package snclient

import (
	"github.com/sni/check_http_go/pkg/checkhttp"
)

func init() {
	AvailableChecks["check_http"] = CheckEntry{"check_http", NewCheckHTTP()}
}

func NewCheckHTTP() *CheckBuiltin {
	return &CheckBuiltin{
		name:        "check_http",
		description: "Runs check_http to perform http(s) checks",
		check:       checkhttp.Check,
	}
}
