package snclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	procFile, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open %s: %s", file, err.Error())
	}
	defer procFile.Close()

	counter := make([]int64, tcpStateMAX-1)
	fileScanner := bufio.NewScanner(procFile)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		fields := strings.Fields(line)
		if fields[3] == "st" {
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
		counter[0]++
		counter[state]++
	}

	return counter, nil
}
