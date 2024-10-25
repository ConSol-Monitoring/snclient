package snclient

import (
	"github.com/sni/check_http_go/pkg/checkhttp"
)

func init() {
	AvailableChecks["check_http"] = CheckEntry{"check_http", NewCheckHTTP}
}

func NewCheckHTTP() CheckHandler {
	return &CheckBuiltin{
		name: "check_http",
		description: `Runs check_http to perform http(s) checks
It basically wraps the plugin from https://github.com/sni/check_http_go`,
		check:    checkhttp.Check,
		docTitle: `check_http`,
		usage:    `check_http [<options>]`,
		exampleDefault: `
Alert if http server does not respond:

    check_http -H omd.consol.de
    HTTP OK - HTTP/1.1 200 OK - 573 bytes in 0.001 second response time | ...

Check for specific string and response code:

    check_http -H omd.consol.de -S -u "/docs/snclient/" -e 200,304 -s "consol" -vvv
    HTTP OK - Status line output "HTTP/2.0 200 OK" matched "200,304", Response body matched "consol"...

It can be a bit tricky to set the -u/--uri on windows, since the / is considered as start of
a command line parameter.

To avoid this issue, simply use the long form --uri=/path.. so the parameter does not start with a slash.
	`,
		exampleArgs: `'-H' 'omd.consol.de' '--uri=/docs' '-S'`,
	}
}
