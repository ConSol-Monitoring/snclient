package main

import (
	"fmt"
	"runtime"
	"testing"
)

var localDaemonBaseINI = `
[/modules]
CheckBuiltinPlugins = enabled
CheckExternalScripts = enabled

[/settings/default]
password = ` + localDaemonPassword + `
certificate = test.crt
certificate key = test.key

[/settings/WEB/server]
use ssl = disabled
port = ` + fmt.Sprintf("%d", localDaemonPort) + `
`

func init() {
	if runtime.GOOS == "windows" {
		localDaemonBaseINI += `
[/settings/external scripts/scripts]
check_echo = cmd /c echo '%ARGS%'
`
	} else {
		localDaemonBaseINI += `
[/settings/external scripts/scripts]
check_echo = echo '%ARGS%'
`
	}
}

func TestDaemonArgsDefault(t *testing.T) {
	bin, _, baseArgs, cleanUp := daemonInit(t, localDaemonBaseINI+`
[/settings/external scripts/alias/echo_alias]
alias = echo_alias
command = check_echo 'te$t'
`)
	defer cleanUp()
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: baseArgs,
		Like: []string{`OK - REST API reachable on http:`},
	})

	// default known arguments are always allowed
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_cpu", "warn=load > 100", "crit=load > 100"),
		Like: []string{`OK - CPU load is ok.`},
	})

	// script arguments are not allowed by default
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_echo", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// alias arguments are not allowed by default
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// argument is set internally from an alias, so it is fine
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias"),
		Like: []string{`te\$t`},
	})

	// additional argument are still not allowed
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})
}

func TestDaemonArgsAliasAllowed(t *testing.T) {
	bin, _, baseArgs, cleanUp := daemonInit(t, localDaemonBaseINI+`
[/settings/external scripts/alias/echo_alias]
alias = echo_alias
command = check_echo 'te$t'

[/settings/external scripts/alias/echo_alias_allowed]
alias = echo_alias_allowed
command = check_echo 'te$t'
allow arguments = true
`)
	defer cleanUp()
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: baseArgs,
		Like: []string{`OK - REST API reachable on http:`},
	})

	// default known arguments are always allowed
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_cpu", "warn=load > 100", "crit=load > 100"),
		Like: []string{`OK - CPU load is ok.`},
	})

	// script arguments are not allowed by default
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_echo", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// alias arguments are not allowed by default
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// argument is set internally from an alias, so it is fine
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias"),
		Like: []string{`te\$t`},
	})

	// additional argument are still not allowed
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// additional argument have been allowed for this one
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias_allowed", "test"),
		Like: []string{`te\$t`},
	})

	// additional argument must not contain nasty chars
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias_allowed", "te$t"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})
}

func TestDaemonArgsScriptsAllowed(t *testing.T) {
	bin, _, baseArgs, cleanUp := daemonInit(t, localDaemonBaseINI+`
[/settings/external scripts]
allow arguments = true
`)
	defer cleanUp()
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: baseArgs,
		Like: []string{`OK - REST API reachable on http:`},
	})

	// default known arguments are always allowed
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_cpu", "warn=load > 100", "crit=load > 100"),
		Like: []string{`OK - CPU load is ok.`},
	})

	// script arguments are allowed in this case
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_echo", "test"),
		Like: []string{`test`},
	})

	// script arguments are not allowed in this case
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_echo", "te$t"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})
}

func TestDaemonArgsScriptsWebForbidden(t *testing.T) {
	bin, _, baseArgs, cleanUp := daemonInit(t, localDaemonBaseINI+`
[/settings/WEB/server]
allow arguments = false

[/settings/external scripts/alias/echo_alias]
alias = echo_alias
command = check_echo 'te$t'

[/settings/external scripts/alias/echo_alias_allowed]
alias = echo_alias_allowed
command = check_echo 'te$t'
allow arguments = true
`)
	defer cleanUp()
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: baseArgs,
		Like: []string{`OK - REST API reachable on http:`},
	})

	// no arguments are ok
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_snclient_version"),
		Like: []string{`SNClient v`},
	})

	// default arguments are forbidden here
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_cpu", "warn=load > 100", "crit=load > 100"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// script arguments are not allowed in this case
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_echo", "test"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// nasty arguments are never allowed
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "check_echo", "te$t"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})

	// additional argument have been allowed for this one
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias_allowed", "test"),
		Like: []string{`te\$t`},
	})

	// additional argument must not contain nasty chars
	runCmd(t, &cmd{
		Cmd:  bin,
		Args: append(baseArgs, "echo_alias_allowed", "te$t"),
		Like: []string{`exception processing request:`},
		Exit: 3,
	})
}
