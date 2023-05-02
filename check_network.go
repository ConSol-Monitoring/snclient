package snclient

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/net"
	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_network"] = CheckEntry{"check_network", new(CheckNetwork)}
}

type CheckNetwork struct {
	noCopy noCopy
	data   CheckData
}

/* check_network
 * Description: Checks the state of network interfaces
 */
func (l *CheckNetwork) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(CheckExitOK)
	var output string
	l.data.detailSyntax = "%(name) >%(sent) <%(received) bps"
	l.data.topSyntax = "%(list)"
	l.data.okSyntax = "%(list)"
	_, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	var checkData map[string]string

	interfaceList, _ := net.Interfaces()
	IOList, _ := net.IOCounters(true)

	metrics := make([]*CheckMetric, 0, len(interfaceList))
	okList := make([]string, 0, len(interfaceList))
	warnList := make([]string, 0, len(interfaceList))
	critList := make([]string, 0, len(interfaceList))

	for intnr, int := range interfaceList {
		mdata := map[string]string{
			"MAC":               int.HardwareAddr,
			"enabled":           strconv.FormatBool(slices.Contains(int.Flags, "up")),
			"name":              int.Name,
			"net_connection_id": int.Name,
			"received":          strconv.FormatUint(IOList[intnr].BytesRecv, 10),
			"sent":              strconv.FormatUint(IOList[intnr].BytesSent, 10),
			"speed":             "-1",
		}

		if CompareMetrics(mdata, l.data.critThreshold) && l.data.critThreshold.value != "none" {
			critList = append(critList, ParseSyntax(l.data.detailSyntax, mdata))

			continue
		}

		if CompareMetrics(mdata, l.data.warnThreshold) && l.data.warnThreshold.value != "none" {
			warnList = append(warnList, ParseSyntax(l.data.detailSyntax, mdata))

			continue
		}

		okList = append(okList, ParseSyntax(l.data.detailSyntax, mdata))
	}

	totalList := append(okList, append(warnList, critList...)...)

	if len(critList) > 0 {
		state = CheckExitCritical
	} else if len(warnList) > 0 {
		state = CheckExitWarning
	}

	checkData = map[string]string{
		"count":        strconv.FormatInt(int64(len(totalList)), 10),
		"ok_list":      strings.Join(okList, ", "),
		"warn_list":    strings.Join(warnList, ", "),
		"crit_list":    strings.Join(critList, ", "),
		"list":         strings.Join(totalList, ", "),
		"problem_list": strings.Join(append(critList, warnList...), ", "),
	}

	if state == CheckExitOK {
		output = ParseSyntax(l.data.okSyntax, checkData)
	} else {
		output = ParseSyntax(l.data.topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
