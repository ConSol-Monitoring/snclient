package snclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"pkg/convert"

	"golang.org/x/exp/slices"
)

func init() {
	AvailableChecks["check_omd"] = CheckEntry{"check_omd", new(CheckOMD)}
}

const (
	defaultOMDCommandTimeout = 30
)

type CheckOMD struct {
	snc           *Agent
	siteFilter    []string
	serviceFilter []string
}

func (l *CheckOMD) Build() *CheckData {
	return &CheckData{
		name:         "check_omd",
		description:  "Check omd site status.",
		hasInventory: ListInventory,
		args: map[string]CheckArgument{
			"site":    {value: &l.siteFilter, isFilter: true, description: "Show this site only"},
			"exclude": {value: &l.serviceFilter, description: "Skip this omd service"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		defaultWarning:  "state == 1",
		defaultCritical: "state >= 2",
		defaultFilter:   "autostart = 1",
		topSyntax:       "${status} - ${list}",
		detailSyntax:    "site ${site}: ${status}${failed_services_txt}",
		emptyState:      3,
		emptySyntax:     "check_omd failed to find any site with this filter.",
	}
}

func (l *CheckOMD) Check(ctx context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.snc = snc
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("check_omd is a linux only command")
	}

	var sites []string
	if len(l.siteFilter) > 0 {
		sites = l.siteFilter
	} else {
		stdout, stderr, _, _, err := snc.runExternalCommandString(ctx, "omd -b sites", defaultOMDCommandTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch omd sites: %s", err.Error())
		}
		if stderr != "" {
			return nil, fmt.Errorf("failed to fetch omd sites: %s", stderr)
		}
		sites = strings.Split(stdout, "\n")
	}

	deadline, ok := ctx.Deadline()
	if !ok || deadline.IsZero() {
		ctxDeadline, cancel := context.WithDeadline(ctx, time.Now().Add(defaultOMDCommandTimeout*time.Second))
		defer cancel()
		ctx = ctxDeadline
	}

	for _, site := range sites {
		l.addOmdSite(ctx, check, site)
	}

	return check.Finalize()
}

func (l *CheckOMD) addOmdSite(ctx context.Context, check *CheckData, site string) {
	details := map[string]string{
		"site":                site,
		"autostart":           "0",
		"state":               "3",
		"status":              "unknown",
		"failed_services":     "",
		"failed_services_txt": "",
	}
	check.listData = append(check.listData, details)

	// check requires root permission
	if os.Geteuid() != 0 {
		details["_error"] = "check_omd requires root permissions"

		return
	}

	if !l.setAutostart(ctx, site, details) {
		return
	}

	statusRaw, stderr, _, _, err := l.snc.runExternalCommandString(ctx, fmt.Sprintf("omd -b status %s", site), defaultOMDCommandTimeout)
	if err != nil {
		log.Warnf("omd status: %s%s", statusRaw, stderr)
		details["_error"] = err.Error()

		return
	}
	states := map[string]int{}
	failed := []string{}
	for _, stateRaw := range strings.Split(statusRaw, "\n") {
		state := strings.Split(stateRaw, " ")
		service := state[0]
		if len(l.serviceFilter) > 0 && slices.Contains(l.serviceFilter, service) {
			continue
		}
		res, err := convert.Float64E(state[1])
		if err != nil {
			details["_error"] = fmt.Sprintf("cannot parse service status: %s (%s)", state[1], err.Error())

			return
		}
		states[service] = int(res)
		if res > 0 && service != "OVERALL" {
			failed = append(failed, service)
		}
	}

	switch {
	case len(failed) == 0:
		details["state"] = "0"
		details["status"] = "running"
	case len(failed) == len(states):
		details["state"] = "2"
		details["status"] = "stopped"
	default:
		details["state"] = "1"
		details["status"] = "partially running"
		details["failed_services"] = strings.Join(failed, ", ")
		details["failed_services_txt"] = ", failed services: " + details["failed_services"]
	}
	l.addLivestatusMetrics(ctx, check, site)
}

func (l *CheckOMD) addLivestatusMetrics(ctx context.Context, check *CheckData, site string) {
	socketPath := fmt.Sprintf("/omd/sites/%s/tmp/run/live", site)
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		log.Debugf("livestatus socket %s does not exist: %s", socketPath, err.Error())

		return
	}
	query := "GET status\nColumns: host_checks_rate service_checks_rate num_hosts num_services\n"
	data, err := l.livestatusQuery(ctx, query, socketPath)
	if err != nil {
		log.Warnf("livestatus query for site %s failed: %s", site, err.Error())

		return
	}
	if len(data) < 1 {
		log.Warnf("livestatus query for site %s failed: result is empty", site)

		return
	}
	row, ok := data[0].([]interface{})
	if !ok {
		log.Warnf("livestatus query for site %s failed", site)

		return
	}
	if len(row) < 3 {
		log.Warnf("livestatus query for site %s failed", site)

		return
	}
	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			Name:     "host_checks_rate",
			Value:    convert.Float64(row[0]),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		},
		&CheckMetric{
			Name:     "service_checks_rate",
			Value:    convert.Float64(row[1]),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		},
		&CheckMetric{
			Name:     "num_hosts",
			Value:    convert.Float64(row[2]),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		},
		&CheckMetric{
			Name:     "num_services",
			Value:    convert.Float64(row[3]),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
		})
}

func (l *CheckOMD) livestatusQuery(ctx context.Context, query, socketPath string) ([]interface{}, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, fmt.Errorf("no deadline set")
	}
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return nil, fmt.Errorf("deadline exceeded")
	}
	conn, err := net.DialTimeout("unix", socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial: %s", err.Error())
	}
	// set read timeout
	err = conn.SetDeadline(deadline)
	if err != nil {
		return nil, fmt.Errorf("conn.SetDeadline: %s", err.Error())
	}

	query = strings.TrimSpace(query)
	query += "\nResponseHeader: fixed16\nOutputFormat: json"
	log.Tracef("sending livestatus query:\n" + query)
	_, err = fmt.Fprintf(conn, "%s\n\n", query)
	if err != nil {
		return nil, fmt.Errorf("socket error: %s", err.Error())
	}

	header := new(bytes.Buffer)
	_, err = io.CopyN(header, conn, 16)
	resBytes := header.Bytes()
	if err != nil {
		return nil, fmt.Errorf("read: %s", err.Error())
	}
	head := bytes.SplitN(resBytes, []byte(" "), 2)
	if len(head) < 2 {
		return nil, fmt.Errorf("response error in livestatus header: %s", resBytes)
	}
	expSize := int64(convert.Float64(string(bytes.TrimSpace(head[1]))))
	body := new(bytes.Buffer)
	_, err = io.CopyN(body, conn, expSize)
	if err != nil && errors.Is(err, io.EOF) {
		err = nil
	}

	if err != nil {
		return nil, fmt.Errorf("io.CopyN: %s", err.Error())
	}

	res := body.Bytes()
	data := make([]interface{}, 0)
	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, fmt.Errorf("json: %s", err.Error())
	}

	return data, nil
}

func (l *CheckOMD) setAutostart(ctx context.Context, site string, details map[string]string) bool {
	autostartRaw, stderr, _, _, err := l.snc.runExternalCommandString(ctx, fmt.Sprintf("omd config %s show AUTOSTART", site), defaultOMDCommandTimeout)
	if err != nil {
		details["_error"] = err.Error()

		return false
	}
	if stderr != "" {
		details["_error"] = stderr

		return false
	}
	if convert.Bool(autostartRaw) {
		details["autostart"] = "1"
	}

	return true
}
