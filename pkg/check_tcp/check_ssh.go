package check_tcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sni/go-flags"
)

func CheckSSH(_ context.Context, output io.Writer, args []string, sendString string) int {
	opts, err := parseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			fmt.Fprint(output, err.Error())

			return 3
		}
		fmt.Fprintf(output, "UNKNOWN - %s", err.Error())

		return 3
	}

	if opts.Port == 0 {
		opts.Port = 22
	}

	if opts.Timeout > opts.Critical && opts.Critical != 0 && opts.Verbose {
		fmt.Fprintf(output, "Timeout to establish connection: %f is higher than critical threshold: %f\n", opts.Timeout, opts.Critical)
	}

	if opts.Timeout > opts.Warning && opts.Warning != 0 && opts.Verbose {
		fmt.Fprintf(output, "Timeout to establish connection: %f is higher than warning threshold: %f\n", opts.Timeout, opts.Warning)
	}

	opts.Send = sendString
	// SSH Tcp connections print out a string:
	// nc -v github.com 22
	// Connection to github.com (140.82.121.4) 22 port [tcp/ssh] succeeded!
	// SSH-2.0-8ad108e

	// RFC 4253:4.2
	// https://datatracker.ietf.org/doc/html/rfc4253 , Page 5
	// The third field consists of any printable ASCII characters
	// EXCEPT whitespaces and minus sign
	// The group at the end consists of two parts
	// !- includes ASCII characters incl. '!' until '-' but not '-'
	// .~ includes ASCII characters incl. '.' until '-' but not '~'
	// this skips over the minus sign
	opts.ExpectPattern = `^SSH-\d+\.\d+-[!-,.-~]+`

	ckr := opts.run(output)
	ckr.Name = "SSH"
	if opts.Service != "" {
		ckr.Name = opts.Service
	}

	fmt.Fprintf(output, "%s %s - %s", ckr.Name, ckr.Status, strings.TrimSpace(ckr.Message))

	return int(ckr.Status)
}
