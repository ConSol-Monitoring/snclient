package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_ping"] = CheckEntry{"check_ping", NewCheckPing}
}

const NastyHostCharacters = "$|`&><'\"\\{}"

type CheckPing struct {
	snc      *Agent
	hostname string
	packets  int64
}

func NewCheckPing() CheckHandler {
	return &CheckPing{
		packets: 5,
	}
}

func (l *CheckPing) Build() *CheckData {
	return &CheckData{
		name:         "check_ping",
		description:  "Checks the icmp ping connection.",
		implemented:  ALL,
		hasInventory: NoInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"host":    {value: &l.hostname, description: "host name or ip address to ping"},
			"packets": {value: &l.packets, description: "number of ICMP ECHO packets to send (default: 5)"},
		},
		defaultFilter:   "none",
		defaultWarning:  "rta > 1000 || pl > 30",
		defaultCritical: "rta > 5000 || pl > 80",
		detailSyntax:    "Packet loss = ${pl}%{{ IF rta != '' }}, RTA = ${rta}ms{{ END }}",
		topSyntax:       "%(status) - ${list}",
		emptyState:      CheckExitUnknown,
		emptySyntax:     "%(status) - could not get any ping data",
		attributes: []CheckAttribute{
			{name: "host_name", description: "host name ping was sent to."},
			{name: "ttl", description: "time to live."},
			{name: "sent", description: "number of packets sent."},
			{name: "received", description: "number of packets received."},
			{name: "rta", description: "average round trip time."},
			{name: "pl", description: "packet loss in percent."},
		},
		exampleDefault: `
    check_ping host=localhost
    OK - Packet loss = 0%, RTA = 0.113ms |...
	`,
		exampleArgs: `'warn=rta > 1000 || pl > 30' 'crit=rta > 5000 || pl > 80'`,
	}
}

func (l *CheckPing) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	if l.hostname == "" {
		return nil, fmt.Errorf("host name is required")
	}
	if strings.ContainsAny(l.hostname, NastyHostCharacters) {
		return nil, fmt.Errorf("host name must not contain nasty characters")
	}

	err := l.addSources(ctx, check)
	if err != nil {
		return nil, err
	}

	return check.Finalize()
}

func (l *CheckPing) addSources(ctx context.Context, check *CheckData) (err error) {
	err = l.addPingLinux(ctx, check)
	if err != nil {
		log.Debugf("failed: ping: %s", err.Error())

		return err
	}

	return nil
}

// run linux ping command
func (l *CheckPing) addPingLinux(ctx context.Context, check *CheckData) error {
	output, stderr, _, err := l.snc.execCommand(ctx, fmt.Sprintf("ping -c %d '%s'", l.packets, l.hostname), DefaultCmdTimeout)
	if err != nil {
		return fmt.Errorf("ping failed: %s\n%s", err.Error(), stderr)
	}

	entry := l.parsePingOutput(output, stderr)
	check.listData = append(check.listData, entry)
	l.addMetrics(check, entry)

	return nil
}

func (l *CheckPing) parsePingOutput(output, stderr string) (entry map[string]string) {
	entry = l.defaultEntry()

	reRTA := regexp.MustCompile(`rtt min/avg/max/mdev = ([\d.]+)/([\d.]+)/([\d.]+)/([\d.]+) ms`)
	rtaList := reRTA.FindStringSubmatch(output)
	if len(rtaList) >= 3 {
		entry["rta"] = rtaList[2]
	}

	reTTL := regexp.MustCompile(`\s+ttl=(\d+)\s+`)
	ttlList := reTTL.FindStringSubmatch(output)
	if len(ttlList) >= 2 {
		entry["ttl"] = ttlList[1]
	}

	rePackets := regexp.MustCompile(`(\d+) packets transmitted, (\d+) received, (\d+)% packet loss`)
	packetsList := rePackets.FindStringSubmatch(output)
	if len(packetsList) >= 4 {
		entry["sent"] = packetsList[1]
		entry["received"] = packetsList[2]
		entry["pl"] = packetsList[3]
	}

	if entry["sent"] == "" {
		rePacketsError := regexp.MustCompile(`(\d+) packets transmitted, (\d+) received, [\+\d]+ errors, (\d+)% packet loss`)
		packetsList = rePacketsError.FindStringSubmatch(output)
		if len(packetsList) >= 4 {
			entry["sent"] = packetsList[1]
			entry["received"] = packetsList[2]
			entry["pl"] = packetsList[3]
		}
	}

	if entry["sent"] == "" {
		if stderr != "" {
			output += stderr
		}
		output = strings.TrimSpace(output)
		// passthrough some known errors
		switch {
		case strings.Contains(output, "Name or service not known"):
			entry["_error"] = output
		default:
			entry["_error"] = fmt.Sprintf("cannot parse ping output: %s", output)
		}
		entry["pl"] = "100"
	}

	return entry
}

func (l *CheckPing) defaultEntry() map[string]string {
	return map[string]string{
		"host_name": l.hostname,
		"ttl":       "",
		"sent":      "",
		"received":  "",
		"rta":       "",
		"pl":        "",
	}
}

func (l *CheckPing) addMetrics(check *CheckData, entry map[string]string) {
	var rta interface{}
	rta = "U"
	if entry["rta"] != "" {
		rta = convert.Float64(entry["rta"])
	}
	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			Name:     "rta",
			Unit:     "ms",
			Value:    rta,
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		},
		&CheckMetric{
			Name:     "pl",
			Value:    convert.Int64(entry["pl"]),
			Unit:     "%",
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
	)
}
