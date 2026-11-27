package snclient

import "github.com/consol-monitoring/snclient/pkg/check_http"

func init() {
	AvailableChecks["check_http"] = CheckEntry{"check_http", NewCheckHTTP}
}

func NewCheckHTTP() CheckHandler {
	return &CheckBuiltin{
		name: "check_http",
		description: `Runs check_http to check HTTP(s) connections.
It wraps the plugin from https://github.com/sni/check_http_go

TLS certificates are not verified by default. This matches monitoring-plugins check_http.
Use <code>--verify-cert</code> to verify the certificate chain and hostname. Use it when the check sends credentials.
Use <code>--ca-cert</code> to verify against a custom CA file. It also enables <code>--verify-cert</code>.
The <code>-C</code> option checks certificate expiry and optional names. By default, it does not verify the chain of trust.`,
		check:    check_http.Check,
		docTitle: `check_http`,
		usage:    `check_http [<options>]`,
		exampleDefault: `
Alert if the HTTP server does not respond:

    check_http -H omd.consol.de
    HTTP OK - HTTP/1.1 200 OK - 573 bytes in 0.001 second response time | ...

Check for a specific string and response code:

    check_http -H omd.consol.de -S -u "/docs/snclient/" -e 200,304 -s "consol" -vvv
    HTTP OK - Status line output "HTTP/2.0 200 OK" matched "200,304", Response body matched "consol"...
	`,
		exampleArgs: `'-H' 'omd.consol.de' '--uri=/docs' '-S'`,
	}
}
