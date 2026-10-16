package snclient

import "github.com/consol-monitoring/snclient/pkg/check_tcp"

func init() {
	AvailableChecks["check_tcp"] = CheckEntry{"check_tcp", NewCheckTCP}
}

func NewCheckTCP() CheckHandler {
	return &CheckBuiltin{
		name: "check_tcp",
		description: `Runs check_tcp to perform TCP connection checks.
It basically wraps the plugin from https://github.com/taku-k/go-check-plugins/tree/master/check-tcp`,
		check:    check_tcp.Check,
		docTitle: `check_tcp`,
		usage:    `check_tcp [<options>]`,
		exampleDefault: `
Alert if TCP connection fails:

    check_tcp -H omd.consol.de -p 80
    TCP OK - 0.003 seconds response time on omd.consol.de port 80

Send something and expect specific string:

    check_tcp -H outlook.com -p 25 -s "HELO" -e "Microsoft ESMTP MAIL Service ready" -q "QUIT"
    TCP OK - 0.197 seconds response time on outlook.com port 25
	`,
		exampleArgs: `'-H' 'omd.consol.de' '-p' '80'`,
	}
}
