package snclient

import (
	"context"
	"strconv"

	"github.com/shirou/gopsutil/v3/net"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_network"] = CheckEntry{"check_network", new(CheckNetwork)}
}

type CheckNetwork struct{}

func (l *CheckNetwork) Build() *CheckData {
	return &CheckData{
		name:        "check_network",
		description: "Checks the state and metrics of network interfaces.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		okSyntax:     "%(status): %(list)",
		detailSyntax: "%(name) >%(sent) <%(received) bps",
		topSyntax:    "%(status): %(list)",
	}
}

func (l *CheckNetwork) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	interfaceList, _ := net.Interfaces()
	IOList, _ := net.IOCounters(true)

	for intnr, int := range interfaceList {
		check.listData = append(check.listData, map[string]string{
			"MAC":               int.HardwareAddr,
			"enabled":           strconv.FormatBool(slices.Contains(int.Flags, "up")),
			"name":              int.Name,
			"net_connection_id": int.Name,
			"received":          strconv.FormatUint(IOList[intnr].BytesRecv, 10),
			"sent":              strconv.FormatUint(IOList[intnr].BytesSent, 10),
			"speed":             "-1",
		})
	}

	return check.Finalize()
}
