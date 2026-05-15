package snclient

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"slices"

	"github.com/consol-monitoring/snclient/pkg/check_tcp"
)

func init() {
	AvailableChecks["check_ssh"] = CheckEntry{"check_ssh", NewCheckSSH}
}

func NewCheckSSH() CheckHandler {
	return &CheckBuiltin{
		name: "check_ssh",
		description: `Runs check_tcp with an SSH configururation to check for a running SSH server.
It basically wraps the plugin from https://github.com/taku-k/go-check-plugins/tree/master/check-tcp`,
		check:    checkSSH,
		docTitle: `check_ssh`,
		usage:    `check_ssh [<options>]`,
		exampleDefault: `
		check_ssh github.com
SSH OK - 0.234 seconds response time on github.com port 22 [SSH-2.0-8ad108e] | time=0.234029s;;;0.000000;10.000000

		check_ssh --hostname github.com --warning 1
SSH OK - 0.262 seconds response time on github.com port 22 [SSH-2.0-8ad108e] | time=0.262048s;;;1.000000;10.000000
	`,
		exampleArgs: `'-H' '192.168.178.100' '-p' '2323'`,
	}
}

func checkSSH(ctx context.Context, output io.Writer, args []string) int {
	// snclient supports short arguments with multiple chars like -v and not -vv or -vvv
	// if snclient agent has these verbose arguments, they are passed as is to internal checks
	// if that is the case, delete them and add -v instead.
	hadVerbose := false
	args = slices.DeleteFunc(args, func(s string) bool {
		isVerbose := s == "-v" || s == "-vv" || s == "-vvv"
		if isVerbose {
			hadVerbose = true
		}

		return isVerbose
	})
	if hadVerbose {
		args = append(args, "-v")
	}

	// the string to be sent on the connection
	sendStr := fmt.Sprintf("SSH-1.0-snclient_build_%s_runtime_%s", Build, runtime.Version())

	return check_tcp.CheckSSH(ctx, output, args, sendStr)
}
