package snclient

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func (l *CheckConnections) addIPV4(_ context.Context, check *CheckData) error {
	counter, err := l.getProcStats("/proc/net/tcp")
	if err != nil {
		return err
	}
	l.addEntry("ipv4", check, counter)

	return nil
}

func (l *CheckConnections) addIPV6(_ context.Context, check *CheckData) error {
	counter, err := l.getProcStats("/proc/net/tcp6")
	if err != nil {
		return err
	}
	l.addEntry("ipv6", check, counter)

	return nil
}

func (l *CheckConnections) getProcStats(file string) ([]int64, error) {
	log.Debugf("collecting stats from %s", file)
	procFile, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open %s: %s", file, err.Error())
	}
	defer procFile.Close()

	counter := make([]int64, tcpStateMAX-1)
	fileScanner := bufio.NewScanner(procFile)
	fileScanner.Scan() // skip first header line
	for fileScanner.Scan() {
		line := fileScanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 4 {
			log.Tracef("corrupt tcp line: %s", line)

			continue
		}
		state, err := strconv.ParseUint(fields[3], 16, 64)
		if err != nil {
			log.Tracef("cannot parse tcp state %s: %s", fields[3], err.Error())

			continue
		}
		if state >= uint64(tcpStateMAX) {
			log.Tracef("unknown tcp state %s", fields[3])

			continue
		}
		if state >= uint64(len(counter)) {
			log.Tracef("tcp state %s out of range", fields[3])

			continue
		}

		counter[0]++
		counter[state]++

		if log.IsV(LogVerbosityTrace2) {
			// debug full entry
			s := tcpStates(convert.UInt16(state))
			fromA, fromP := l.parseHexIP(fields[1])
			toA, toP := l.parseHexIP(fields[2])
			log.Tracef("from: %30s:%-7d to: %30s:%-7d uid: %6s state: %s", fromA, fromP, toA, toP, fields[5], s.String())
		}
	}

	return counter, nil
}

func (l *CheckConnections) parseHexIP(raw string) (address string, port uint64) {
	fields := strings.Split(raw, ":")
	if len(fields) != 2 {
		return raw, 0
	}

	port, err := strconv.ParseUint(fields[1], 16, 16)
	if err != nil {
		log.Tracef("port parse error for address %s: %s", raw, err.Error())

		return raw, 0
	}

	ipBytes, err := hex.DecodeString(fields[0])
	if err != nil {
		log.Tracef("ip parse error for address %s: %s", raw, err.Error())

		return raw, 0
	}

	// Reverse bytes because the IP is stored in little-endian
	for i, j := 0, len(ipBytes)-1; i < j; i, j = i+1, j-1 {
		ipBytes[i], ipBytes[j] = ipBytes[j], ipBytes[i]
	}

	// Convert to an IP address
	ip := net.IP(ipBytes)

	return ip.String(), port
}
