package snclient

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/net"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_network"] = CheckEntry{"check_network", new(CheckNetwork)}
}

type CheckNetwork struct {
	names    []string
	excludes []string
}

func (l *CheckNetwork) Build() *CheckData {
	l.names = []string{}
	l.excludes = []string{}

	return &CheckData{
		name:         "check_network",
		description:  "Checks the state and metrics of network interfaces.",
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"dev":     &l.names,
			"device":  &l.names,
			"name":    &l.names,
			"exclude": &l.excludes,
		},
		okSyntax:     "%(status): %(list)",
		detailSyntax: "%(name) >%(sent) <%(received) bps",
		topSyntax:    "%(status): %(list)",
		emptySyntax:  "%(status): No devices found",
		emptyState:   CheckExitUnknown,
	}
}

func (l *CheckNetwork) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
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

		speed := "-1"
		// grab speed from /sys/class/net/<dev>/speed if possible
		if runtime.GOOS == "linux" {
			dat, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/speed", int.Name))
			if err == nil {
				speed = strings.TrimSpace(string(dat))
			}
		}

		check.listData = append(check.listData, map[string]string{
			"MAC":               int.HardwareAddr,
			"enabled":           strconv.FormatBool(slices.Contains(int.Flags, "up")),
			"name":              int.Name,
			"net_connection_id": int.Name,
			"received":          strconv.FormatUint(IOList[intnr].BytesRecv, 10),
			"sent":              strconv.FormatUint(IOList[intnr].BytesSent, 10),
			"speed":             speed,
			"flags":             strings.Join(int.Flags, ","),
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
				"sent":              "0",
				"speed":             "-1",
				"flags":             "",
			})
		}
	}

	return check.Finalize()
}
