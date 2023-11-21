package snclient

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pkg/humanize"

	"github.com/shirou/gopsutil/v3/net"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_network"] = CheckEntry{"check_network", new(CheckNetwork)}
}

type CheckNetwork struct {
	snc      *Agent
	names    []string
	excludes []string
}

func (l *CheckNetwork) Build() *CheckData {
	l.names = []string{}
	l.excludes = []string{}

	return &CheckData{
		name:         "check_network",
		description:  "Checks the state and metrics of network interfaces.",
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"dev":     {value: &l.names, description: "Alias for device"},
			"device":  {value: &l.names, description: "The device to check. Default is all"},
			"name":    {value: &l.names, description: "Alias for device"},
			"exclude": {value: &l.excludes, description: "Exclude device by name"},
		},
		defaultWarning:  "total > 10000",
		defaultCritical: "total > 100000",
		okSyntax:        "%(status): %(list)",
		detailSyntax:    "%(name) >%(sent) <%(received)",
		topSyntax:       "%(status): %(list)",
		emptySyntax:     "%(status): No devices found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "MAC", description: "The MAC address"},
			{name: "enabled", description: "True if the network interface is enabled (true/false)"},
			{name: "name", description: "Name of the interface"},
			{name: "net_connection_id", description: "same as name"},
			{name: "received", description: "Bytes received per second"},
			{name: "total_received", description: "Total bytes received"},
			{name: "sent", description: "Bytes sent per second"},
			{name: "total_sent", description: "Total bytes sent"},
			{name: "speed", description: "Network interface speed"},
			{name: "flags", description: "Interface flags"},
			{name: "total", description: "Sum of sent and received bytes per second"},
		},
		exampleDefault: `
    check_network device=eth0
    OK: eth0 >12 kB/s <28 kB/s |...
	`,
	}
}

func (l *CheckNetwork) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	interfaceList, _ := net.Interfaces()
	IOList, err := net.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("net.IOCounters: %s", err.Error())
	}

	found := map[string]bool{}
	for intnr, int := range interfaceList {
		if slices.Contains(l.excludes, int.Name) {
			log.Tracef("device %s excluded by 'exclude' argument", int.Name)

			continue
		}
		if len(l.names) > 0 && !slices.Contains(l.names, int.Name) {
			log.Tracef("device %s excluded by 'name' argument", int.Name)

			continue
		}
		found[int.Name] = true

		speed, err := l.interfaceSpeed(int.Index, int.Name)
		if err != nil {
			log.Debugf("failed to get interface speed for %s: %s", int.Name, err.Error())
			speed = -1
		}

		recvRate := float64(0)
		recv1 := l.snc.Counter.Get("net", int.Name+"_recv").GetLast()
		recv2 := l.snc.Counter.Get("net", int.Name+"_recv").GetAt(time.Now().Add(-30 * time.Second))
		if recv1 != nil && recv2 != nil && recv1.timestamp.After(recv2.timestamp) {
			recvRate = recv1.value - recv2.value/float64(recv1.timestamp.Unix()) - float64(recv2.timestamp.Unix())
		}
		if recvRate < 0 {
			recvRate = 0
		}
		sentRate := float64(0)
		sent1 := l.snc.Counter.Get("net", int.Name+"_sent").GetLast()
		sent2 := l.snc.Counter.Get("net", int.Name+"_sent").GetAt(time.Now().Add(-30 * time.Second))
		if sent1 != nil && sent2 != nil && sent1.timestamp.After(sent2.timestamp) {
			sentRate = sent1.value - sent2.value/float64(sent1.timestamp.Unix()) - float64(sent2.timestamp.Unix())
		}
		if sentRate < 0 {
			sentRate = 0
		}

		check.listData = append(check.listData, map[string]string{
			"MAC":               int.HardwareAddr,
			"enabled":           strconv.FormatBool(slices.Contains(int.Flags, "up")),
			"name":              int.Name,
			"net_connection_id": int.Name,
			"received":          humanize.IBytes(uint64(recvRate)) + "/s",
			"total_received":    fmt.Sprintf("%d", IOList[intnr].BytesRecv),
			"sent":              humanize.IBytes(uint64(sentRate)) + "/s",
			"total_sent":        fmt.Sprintf("%d", IOList[intnr].BytesSent),
			"total":             fmt.Sprintf("%.2f", recvRate+sentRate),
			"speed":             fmt.Sprintf("%d", speed),
			"flags":             strings.Join(int.Flags, ","),
		})
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			ThresholdName: int.Name,
			Name:          fmt.Sprintf("%s_traffic_in", int.Name),
			Value:         IOList[intnr].BytesRecv,
			Unit:          "c",
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
		})
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			ThresholdName: int.Name,
			Name:          fmt.Sprintf("%s_traffic_out", int.Name),
			Value:         IOList[intnr].BytesSent,
			Unit:          "c",
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
		})
	}

	// warn about all interfaces explicitly requested but not found
	for _, deviceName := range l.names {
		if _, ok := found[deviceName]; !ok {
			check.listData = append(check.listData, map[string]string{
				"_error":            fmt.Sprintf("no device named %s found", deviceName),
				"MAC":               "",
				"enabled":           "false",
				"name":              deviceName,
				"net_connection_id": deviceName,
				"received":          "0",
				"toal_received":     "0",
				"sent":              "0",
				"total_sent":        "0",
				"total":             "0",
				"speed":             "-1",
				"flags":             "",
			})
		}
	}

	return check.Finalize()
}
