package snclient

import (
	"fmt"
	"net"

	"pkg/nrpe"
)

const (
	nrpeQueryPacketType    = 1
	nrpeResponsePacketType = 2
)

func init() {
	AvailableListeners = append(AvailableListeners, ListenHandler{"NRPEServer", "/settings/NRPE/server", NewHandlerNRPE()})
}

type HandlerNRPE struct {
	noCopy noCopy
	snc    *Agent
}

func NewHandlerNRPE() *HandlerNRPE {
	l := &HandlerNRPE{}

	return l
}

func (l *HandlerNRPE) Type() string {
	return "nrpe"
}

func (l *HandlerNRPE) Defaults() ConfigSection {
	defaults := ConfigSection{
		"allow arguments":   "0",
		"allow nasty chars": "0",
		"port":              "5666",
	}
	defaults.Merge(DefaultListenTCPConfig)

	return defaults
}

func (l *HandlerNRPE) Init(snc *Agent) error {
	l.snc = snc

	return nil
}

func (l *HandlerNRPE) ServeTCP(snc *Agent, con net.Conn) {
	defer con.Close()

	request := nrpe.NewNrpePacket()

	if err := request.Read(con); err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	if err := request.Verify(nrpeQueryPacketType); err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	cmd, args := request.Data()
	log.Tracef("nrpe v%d request: %s %s", request.Version(), cmd, args)

	var statusResult *CheckResult

	// version check
	if cmd == "_NRPE_CHECK" {
		statusResult = &CheckResult{
			State:  0,
			Output: fmt.Sprintf("%s v%s.%s", NAME, VERSION, snc.Revision),
		}
	} else {
		statusResult = snc.RunCheck(cmd, args)
	}

	output := []byte(statusResult.Output)
	if len(statusResult.Metrics) > 0 {
		output = append(output, '|')

		for _, m := range statusResult.Metrics {
			output = append(output, []byte(m.BuildNaemonString())...)
		}
	}

	response := nrpe.BuildPacketV2(nrpeResponsePacketType, uint16(statusResult.State), output)

	if err := response.Write(con); err != nil {
		log.Errorf("nrpe write response error: %s", err.Error())

		return
	}
}
