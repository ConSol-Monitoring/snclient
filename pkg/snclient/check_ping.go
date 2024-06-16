package snclient

import (
	"context"
	"fmt"
	"runtime"
)

func init() {
	AvailableChecks["check_ping"] = CheckEntry{"check_ping", NewCheckPing}
}

type CheckPing struct {
	snc     *Agent
	hosts   []string
	timeout int64
	count   int64
}

func NewCheckPing() CheckHandler {
	return &CheckPing{
		hosts:   []string{},
		timeout: 500,
		count:   1,
	}
}

func (l *CheckPing) Build() *CheckData {
	return &CheckData{
		name:         "check_ping",
		description:  "Checks a remote host availability.",
		implemented:  ALL,
		hasInventory: NoInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"host":    {value: &l.hosts, description: "The remote host to ping."},
			"hosts":   {value: &l.hosts, description: "Alias for host."},
			"timeout": {value: &l.timeout, description: "Ping timeout in milliseconds. Default: 500ms"},
			"count":   {value: &l.count, description: "Number of packets to send. Default: 1"},
		},
		defaultWarning:  "time > 60 or loss > 5%",
		defaultCritical: "time > 100 or loss > 10%",
		okSyntax:        "%(status): All %(count) hosts are ok",
		detailSyntax:    "${ip} Packet loss = ${loss}%, RTA = ${time}ms",
		topSyntax:       "${status}: ${ok_count}/${count} (${problem_list})",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "No hosts found",
		attributes: []CheckAttribute{
			{name: "host", description: "The host name or ip address (as given on command line)"},
			{name: "ip", description: "The resolved ip address"},
			{name: "loss", description: "Packet loss (in percent)"},
			{name: "recv", description: "Number of packets received from the host"},
			{name: "sent", description: "Number of packets sent to the host"},
			{name: "timeout", description: "Number of packets which timed out from the host"},
			{name: "time", description: "Round trip time in ms"},
		},
		exampleDefault: `
    check_ping host=omd.consol.de
    OK - TODO: ...
	`,
		exampleArgs: `host=omd.consol.de timeout=100 count=2`,
	}
}

func (l *CheckPing) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckNet")
	if !enabled {
		return nil, fmt.Errorf("module CheckNet is not enabled in /modules section")
	}

	if len(l.hosts) == 0 {
		return nil, fmt.Errorf("must specify at least one host")
	}

	switch runtime.GOOS {
	case "linux":
		for _, host := range l.hosts {
			err := l.addPingLinux(ctx, check, host)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("os %s is not supported", runtime.GOOS)
	}

	return check.Finalize()
}

func (l *CheckPing) addPingLinux(ctx context.Context, check *CheckData, host string) error {
	output, stderr, _, err := l.snc.execCommand(ctx, fmt.Sprintf("ping %s -c %d -W %d", host, l.count, l.timeout), DefaultCmdTimeout)
	if err != nil {
		return fmt.Errorf("ping failed: %s\n%s", err.Error(), stderr)
	}

	log.Debugf("%s", output)

	return nil
}
