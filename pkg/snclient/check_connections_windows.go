package snclient

import (
	"context"
	"fmt"

	winnetstat "github.com/pytimer/win-netstat"
)

// get open tcp connections from the windows iphlpapi via win-netstat library
func (l *CheckConnections) addIPV4(_ context.Context, check *CheckData) error {
	counter, err := l.getNetstat("tcp4")
	if err != nil {
		return err
	}
	l.addEntry("ipv4", check, counter)

	return nil
}

func (l *CheckConnections) addIPV6(_ context.Context, check *CheckData) error {
	counter, err := l.getNetstat("tcp6")
	if err != nil {
		return err
	}
	l.addEntry("ipv6", check, counter)

	return nil
}

func (l *CheckConnections) getNetstat(kind string) ([]uint64, error) {
	connections, err := winnetstat.Connections(kind)
	if err != nil {
		return nil, fmt.Errorf("fetching %s connections failed with error: %s", kind, err.Error())
	}

	counter := make([]uint64, tcpStateMAX-1)

	for idx := range connections {
		// available states: https://learn.microsoft.com/en-us/windows/win32/api/tcpmib/ne-tcpmib-mib_tcp_state
		// State object is saved as english string, converted in win-netstat\common.go
		switch connections[idx].State {
		case "CLOSE_WAIT":
			counter[tcpCloseWait]++
		// Deleted counts as closed as well
		case "CLOSED", "DELETE":
			counter[tcpClose]++
		case "CLOSING":
			counter[tcpClosing]++
		case "ESTABLISHED":
			counter[tcpEstablished]++
		case "FIN_WAIT_1":
			counter[tcpFinWait1]++
		case "FIN_WAIT_2":
			counter[tcpFinWait2]++
		case "LAST_ACK":
			counter[tcpLastAck]++
		case "LISTEN":
			counter[tcpListen]++
		case "SYN_RECEIVED":
			counter[tcpSynRecv]++
		case "SYN_SENT":
			counter[tcpSynSent]++
		case "TIME_WAIT":
			counter[tcpTimeWait]++
		default:
			log.Tracef("unknown tcp state: %s", connections[idx].State)
		}
		counter[tcpTotal]++
	}

	return counter, nil
}
