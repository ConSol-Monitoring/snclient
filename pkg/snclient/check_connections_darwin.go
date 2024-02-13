package snclient

import (
	"context"
	"fmt"
	"strings"
)

// get open tcp connections from netstat.exe
func (l *CheckConnections) addIPV4(ctx context.Context, check *CheckData) error {
	counter, err := l.getNetstat(ctx, "inet")
	if err != nil {
		return err
	}
	l.addEntry("ipv4", check, counter)

	return nil
}

func (l *CheckConnections) addIPV6(ctx context.Context, check *CheckData) error {
	counter, err := l.getNetstat(ctx, "inet6")
	if err != nil {
		return err
	}
	l.addEntry("ipv6", check, counter)

	return nil
}

func (l *CheckConnections) addEntry(name string, check *CheckData, counter []int64) {
	entry := l.defaultEntry(name)
	for i := range counter {
		s := tcpStates(i)
		entry[s.String()] = fmt.Sprintf("%d", counter[i])
	}

	check.listData = append(check.listData, entry)
}

func (l *CheckConnections) getNetstat(ctx context.Context, name string) ([]int64, error) {
	output, stderr, rc, err := l.snc.runExternalCommandString(ctx, "netstat -an -p tcp -f  "+name, DefaultCmdTimeout)
	if err != nil {
		return nil, fmt.Errorf("netstat failed: %s\n%s", err.Error(), stderr)
	}
	if rc != 0 {
		return nil, fmt.Errorf("netstat failed: %s\n%s", output, stderr)
	}

	counter := make([]int64, tcpStateMAX-1)

	for _, line := range strings.Split(output, "\n") {
		cols := strings.Fields(line)
		if len(cols) < 6 {
			continue
		}
		if !strings.HasPrefix(cols[0], "tcp") {
			continue
		}

		// tcp4       0      0  127.0.0.1.8021         *.*                    LISTEN
		// tcp4       0      0  127.0.0.1.50410        127.0.0.1.9990         ESTABLISHED
		// tcp4       0      0  127.0.0.1.9990         *.*                    LISTEN
		// tcp46      0      0  *.8443                 *.*                    LISTEN
		switch cols[5] {
		case "CLOSE_WAIT":
			counter[tcpCloseWait]++
		case "CLOSED":
			counter[tcpClose]++
		case "ESTABLISHED":
			counter[tcpEstablished]++
		case "FIN_WAIT_1":
			counter[tcpFinWait1]++
		case "FIN_WAIT_2":
			counter[tcpFinWait2]++
		case "LAST_ACK":
			counter[tcpLastAck]++
		case "LISTEN", "LISTENING":
			counter[tcpListen]++
		case "SYN_RECEIVED":
			counter[tcpSynRecv]++
		case "SYN_SEND":
			counter[tcpSynSent]++
		case "TIMED_WAIT", "TIME_WAIT":
			counter[tcpTimeWait]++
		default:
			log.Errorf("unhandled tcp state: %s", cols[5])
		}
		counter[tcpTotal]++
	}

	return counter, nil
}
