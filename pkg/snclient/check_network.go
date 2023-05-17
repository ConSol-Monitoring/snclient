package snclient

import (
	"strconv"

	"github.com/shirou/gopsutil/v3/net"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_network"] = CheckEntry{"check_network", new(CheckNetwork)}
}

type CheckNetwork struct{}

/* check_network
 * Description: Checks the state of network interfaces
 */
func (l *CheckNetwork) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		okSyntax:     "%(list)",
		detailSyntax: "%(name) >%(sent) <%(received) bps",
		topSyntax:    "%(list)",
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

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
