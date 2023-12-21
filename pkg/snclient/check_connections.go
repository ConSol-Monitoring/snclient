//go:build linux || windows || darwin

package snclient

import (
	"context"
	"fmt"

	"pkg/convert"
)

func init() {
	AvailableChecks["check_connections"] = CheckEntry{"check_connections", NewCheckConnections}
}

// tcp states as defined in https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/net/tcp_states.h
type tcpStates uint8

const (
	tcpTotal tcpStates = iota
	tcpEstablished
	tcpSynSent
	tcpSynRecv
	tcpFinWait1
	tcpFinWait2
	tcpTimeWait
	tcpClose
	tcpCloseWait
	tcpLastAck
	tcpListen
	tcpClosing
	tcpNewSynRecv
	tcpStateMAX // pseudo entry, keep this last in case new states would be added
)

func (t *tcpStates) String() string {
	switch *t {
	case tcpTotal:
		return "total"
	case tcpEstablished:
		return "established"
	case tcpSynSent:
		return "syn_sent"
	case tcpSynRecv:
		return "syn_recv"
	case tcpFinWait1:
		return "fin_wait1"
	case tcpFinWait2:
		return "fin_wait2"
	case tcpTimeWait:
		return "time_wait"
	case tcpClose:
		return "close"
	case tcpCloseWait:
		return "close_wait"
	case tcpLastAck:
		return "last_ack"
	case tcpListen:
		return "listen"
	case tcpClosing:
		return "closing"
	case tcpNewSynRecv:
		return "new_syn_recv"
	case tcpStateMAX:
		return "max"
	}

	return "unknown"
}

type CheckConnections struct {
	snc           *Agent
	addressFamily string
}

func NewCheckConnections() CheckHandler {
	return &CheckConnections{}
}

func (l *CheckConnections) Build() *CheckData {
	return &CheckData{
		name:         "check_connections",
		description:  "Checks the number of tcp connections.",
		implemented:  Linux | Windows | Darwin,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"inet": {value: &l.addressFamily, isFilter: true, description: "Use specific address family only. Can be: total, any, ipv4 or ipv6"},
		},
		defaultFilter:   "inet=total",
		defaultWarning:  "total > 500",
		defaultCritical: "total > 1500",
		detailSyntax:    "total ${prefix}connections: ${total}",
		topSyntax:       "${status}: ${list}",
		attributes: []CheckAttribute{
			{name: "inet", description: "address family, can be total (sum of any), all (any+total), any (v4+v6), inet4 or inet6"},
			{name: "prefix", description: "address family as prefix, will be empty, inet4 or inet6"},
			{name: "total", description: "total number of connections"},
			{name: "established", description: "total number of connections of type: established"},
			{name: "syn_sent", description: "total number of connections of type: syn_sent"},
			{name: "syn_recv", description: "total number of connections of type: syn_recv"},
			{name: "fin_wait1", description: "total number of connections of type: fin_wait1"},
			{name: "fin_wait2", description: "total number of connections of type: fin_wait2"},
			{name: "time_wait", description: "total number of connections of type: time_wait"},
			{name: "close", description: "total number of connections of type: close"},
			{name: "close_wait", description: "total number of connections of type: close_wait"},
			{name: "last_ack", description: "total number of connections of type: last_ack"},
			{name: "listen", description: "total number of connections of type: listen"},
			{name: "closing", description: "total number of connections of type: closing"},
			{name: "new_syn_recv", description: "total number of connections of type: new_syn_recv"},
		},
		exampleDefault: `
    check_connections
    OK: total connections 60

Check only ipv6 connections:

    check_connections inet=ipv6
    OK: total ipv6 connections 13
	`,
		exampleArgs: `'warn=total > 500' 'crit=total > 1500'`,
	}
}

func (l *CheckConnections) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc
	if l.addressFamily == "" {
		l.addressFamily = "all"
	}
	if l.addressFamily != "all" && l.addressFamily != "total" && l.addressFamily != "any" && l.addressFamily != "ipv4" && l.addressFamily != "ipv6" {
		return nil, fmt.Errorf("option inet must be all, any, ipv4 or ipv6")
	}

	if l.addressFamily != "ipv6" {
		err := l.addIPV4(ctx, check)
		if err != nil {
			return nil, err
		}
	}
	if l.addressFamily != "ipv4" {
		err := l.addIPV6(ctx, check)
		if err != nil {
			return nil, err
		}
	}

	if l.addressFamily == "total" || l.addressFamily == "all" {
		// combine all into a total
		entry := l.defaultEntry("total")
		for i := range check.listData {
			row := check.listData[i]
			for key := range row {
				num1 := convert.Int64(row[key])
				num2 := convert.Int64(entry[key])
				entry[key] = fmt.Sprintf("%d", num1+num2)
			}
		}
		entry["inet"] = "total"
		entry["prefix"] = ""
		if l.addressFamily == "total" {
			check.listData = []map[string]string{entry}
		} else {
			check.listData = append([]map[string]string{entry}, check.listData...)
		}
	}

	// create metrics
	for i := range check.listData {
		entry := check.listData[i]
		if entry["inet"] != "total" {
			entry["prefix"] = entry["inet"] + " "
		}
		check.listData[i] = entry
		if check.MatchMapCondition(check.filter, entry, false) {
			l.addMetrics(check, entry)
		}
	}

	return check.Finalize()
}

func (l *CheckConnections) defaultEntry(source string) map[string]string {
	entry := map[string]string{
		"inet":   source,
		"prefix": "",
	}
	for i := tcpStates(0); i < tcpStateMAX; i++ {
		name := i.String()
		entry[name] = ""
	}

	return entry
}

func (l *CheckConnections) addMetrics(check *CheckData, entry map[string]string) {
	prefix := ""
	if l.addressFamily == "any" {
		prefix = entry["inet"] + "_"
	}
	for i := tcpStates(0); i < tcpStateMAX; i++ {
		name := i.String()
		str, ok := entry[name]
		if !ok {
			continue
		}
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: name,
				Name:          prefix + name,
				Unit:          "",
				Value:         convert.Int64(str),
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			},
		)
	}
}
