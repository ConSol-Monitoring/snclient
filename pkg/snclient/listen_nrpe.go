package snclient

import (
	"context"
	"net"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/nrpe"
)

func init() {
	RegisterModule(
		&AvailableListeners,
		"NRPEServer",
		"/settings/NRPE/server",
		NewHandlerNRPE,
		ConfigInit{
			ConfigData{
				"port":                   "5666",
				"use ssl":                "true",
				"allow arguments":        "false",
				"allow nasty characters": "false",
			},
			"/settings/default",
			DefaultListenTCPConfig,
		},
	)
}

type HandlerNRPE struct {
	noCopy       noCopy
	snc          *Agent
	conf         *ConfigSection
	listener     *Listener
	allowedHosts *AllowedHostConfig
}

// ensure we fully implement the RequestHandlerTCP type
var _ RequestHandlerTCP = &HandlerNRPE{}

func NewHandlerNRPE() Module {
	return &HandlerNRPE{}
}

func (l *HandlerNRPE) Init(snc *Agent, conf *ConfigSection, _ *Config, _ *AgentRunSet) error {
	l.snc = snc
	l.conf = conf
	listener, err := NewListener(snc, conf, l)
	if err != nil {
		return err
	}
	l.listener = listener

	allowedHosts, err := NewAllowedHostConfig(conf)
	if err != nil {
		return err
	}
	l.allowedHosts = allowedHosts

	return nil
}

func (l *HandlerNRPE) GetAllowedHosts() *AllowedHostConfig {
	return l.allowedHosts
}

func (l *HandlerNRPE) Type() string {
	return "nrpe"
}

func (l *HandlerNRPE) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerNRPE) Listener() *Listener {
	return l.listener
}

func (l *HandlerNRPE) Start() error {
	return l.listener.Start()
}

func (l *HandlerNRPE) Stop() {
	if l.listener != nil {
		l.listener.Stop()
	}
}

func (l *HandlerNRPE) ServeTCP(snc *Agent, con net.Conn) {
	defer con.Close()

	request, err := nrpe.ReadNrpePacket(con)
	if err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	if err := request.Verify(nrpe.NrpeQueryPacket); err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	cmd, args := request.Data()
	log.Tracef("nrpe v%d request: %s %#v", request.Version(), cmd, args)

	if request.Version() == nrpe.NrpeV3PacketVersion {
		log.Errorf("nrpe protocol version 3 is deprecated, use v2 or v4")

		return
	}

	if cmd == "_NRPE_CHECK" {
		// version check
		cmd = "check_snclient_version"
		args = []string{}
	}
	statusResult := snc.RunCheckWithContext(context.TODO(), cmd, args, 0, l.conf)

	output := statusResult.BuildPluginOutput()
	state, err2 := convert.UInt16E(statusResult.State)
	if err2 != nil {
		log.Errorf("failed to convert exit code %d: %s", statusResult.State, err2.Error())
		state = 3
	}
	response := nrpe.BuildPacket(request.Version(), nrpe.NrpeResponsePacket, state, output)

	if err := response.Write(con); err != nil {
		log.Errorf("nrpe write response error: %s", err.Error())

		return
	}
}
