package snclient

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/shirou/gopsutil/v4/net"
)

func init() {
	AvailableChecks["check_network"] = CheckEntry{"check_network", NewCheckNetwork}
}

const (
	TrafficRateDuration = 30 * time.Second
)

type CheckNetwork struct {
	snc      *Agent
	names    []string
	excludes []string
}

func NewCheckNetwork() CheckHandler {
	return &CheckNetwork{}
}

func (l *CheckNetwork) Build() *CheckData {
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
		defaultWarning:  "total > 80GB",
		defaultCritical: "total > 90GB",
		okSyntax:        "%(status) - %(list)",
		detailSyntax:    "%(name) >%(sent) <%(received)",
		topSyntax:       "%(status) - %(list)",
		emptySyntax:     "%(status) - No devices found",
		emptyState:      CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "MAC", description: "The MAC address"},
			{name: "enabled", description: "True if the network interface is enabled (true/false)"},
			{name: "name", description: "Name of the interface"},
			{name: "net_connection_id", description: "same as name"},
			{name: "received", description: "Bytes received per second (calculated over the last " + TrafficRateDuration.String() + ")"},
			{name: "total_received", description: "Total bytes received"},
			{name: "sent", description: "Bytes sent per second (calculated over the last " + TrafficRateDuration.String() + ")"},
			{name: "total_sent", description: "Total bytes sent"},
			{name: "speed", description: "Network interface speed (in Mbits/sec)"},
			{name: "flags", description: "Interface flags"},
			{name: "total", description: "Sum of sent and received bytes per second"},
		},
		exampleDefault: `
    check_network device=eth0
    OK - eth0 >12 kB/s <28 kB/s |...
	`,
	}
}

func (l *CheckNetwork) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc

	interfaceList, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("net.Interfaces: %s", err.Error())
	}
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

		recvRate, sentRate := l.getTrafficRates(int.Name)

		totalReceived := uint64(0)
		totalSent := uint64(0)
		if len(IOList) <= intnr {
			log.Debugf("failed to get interface counters for %s, only have %d io counters", int.Name, len(IOList))
		} else {
			totalReceived = IOList[intnr].BytesRecv
			totalSent = IOList[intnr].BytesSent
		}

		entry := map[string]string{
			"MAC":               int.HardwareAddr,
			"enabled":           strconv.FormatBool(slices.Contains(int.Flags, "up")),
			"name":              int.Name,
			"net_connection_id": int.Name,
			"received":          humanize.IBytes(uint64(recvRate)) + "/s",
			"total_received":    fmt.Sprintf("%d", totalReceived),
			"sent":              humanize.IBytes(uint64(sentRate)) + "/s",
			"total_sent":        fmt.Sprintf("%d", totalSent),
			"total":             fmt.Sprintf("%.2f", recvRate+sentRate),
			"speed":             fmt.Sprintf("%d", speed),
			"flags":             strings.Join(int.Flags, ","),
		}
		if speed == -1 {
			entry["speed"] = ""
		}

		if !check.MatchMapCondition(check.filter, entry, true) {
			log.Tracef("device %s excluded by filter", int.Name)

			continue
		}

		check.listData = append(check.listData, entry)

		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			ThresholdName: int.Name,
			Name:          fmt.Sprintf("%s_traffic_in", int.Name),
			Value:         totalReceived,
			Unit:          "c",
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
		})
		check.result.Metrics = append(check.result.Metrics, &CheckMetric{
			ThresholdName: int.Name,
			Name:          fmt.Sprintf("%s_traffic_out", int.Name),
			Value:         totalSent,
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

func (l *CheckNetwork) getTrafficRates(name string) (received, sent float64) {
	received, _ = l.snc.Counter.GetRate("net", name+"_recv", TrafficRateDuration)
	sent, _ = l.snc.Counter.GetRate("net", name+"_sent", TrafficRateDuration)

	if received < 0 {
		received = 0
	}

	if sent < 0 {
		sent = 0
	}

	return
}
