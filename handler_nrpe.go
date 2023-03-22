package snclient

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	nrpeMaxPacketDataLength = 1024
	nrpePacketLength        = nrpeMaxPacketDataLength + 12
	nrpeQueryPacketType     = 1
	nrpeResponsePacketType  = 2
	// currently supporting latest version2 protocol.
	// TODO: add v3/4
	nrpePacketVersion2 = 2
)

type HandlerNRPE struct {
	noCopy noCopy
}

func NewHandlerNRPE() *HandlerNRPE {
	l := &HandlerNRPE{}

	return l
}

func (l *HandlerNRPE) Type() string {
	return "nrpe"
}

func (l *HandlerNRPE) Defaults() map[string]string {
	defaults := map[string]string{
		"port":                   "5666",
		"command_timeout":        "60",
		"allow_arguments":        "0",
		"allow_nasty_meta_chars": "0",
		"use_ssl":                "0",
		"bind_to_address":        "",
		"allowed_hosts":          "",
	}

	return defaults
}

func (l *HandlerNRPE) Init(_ *Agent) error {
	return nil
}

func (l *HandlerNRPE) ServeTCP(_ *Agent, con net.Conn) {
	defer con.Close()

	request := NewNrpePacket()

	if err := request.Read(con); err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	if err := request.Verify(nrpeQueryPacketType); err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	pos := bytes.IndexByte(request.data, 0)

	if pos == -1 {
		log.Errorf("nrpe: invalid request")

		return
	}

	data := strings.Split(string(request.data[:pos]), "!")
	log.Tracef("nrpe v%d request: %s %s", binary.BigEndian.Uint16(request.packetVersion), data[0], data[1:])

	statusCode := 0
	statusOutput := fmt.Sprintf("OK - got nrpe request - cmd: %s args: %s", data[0], data[1:])

	response := l.buildPacket(nrpeResponsePacketType, uint16(statusCode), []byte(statusOutput))

	if err := response.Write(con); err != nil {
		log.Errorf("nrpe write response error: %s", err.Error())

		return
	}
}

// buildPacket creates packet structure.
func (l *HandlerNRPE) buildPacket(packetType uint16, statusCode uint16, statusLine []byte) *nrpePacket {
	packet := NewNrpePacket()

	binary.BigEndian.PutUint16(packet.packetVersion, nrpePacketVersion2)
	binary.BigEndian.PutUint16(packet.packetType, packetType)
	binary.BigEndian.PutUint32(packet.crc32, 0)
	binary.BigEndian.PutUint16(packet.statusCode, statusCode)

	length := len(statusLine)

	if length >= nrpeMaxPacketDataLength {
		length = nrpeMaxPacketDataLength - 1
	}

	copy(packet.data, statusLine[:length])
	packet.data[length] = 0

	binary.BigEndian.PutUint32(packet.crc32, packet.BuildCRC32())

	return packet
}

// nrpePacket stores nrpe request / response packet.
type nrpePacket struct {
	packetVersion []byte
	packetType    []byte
	crc32         []byte
	statusCode    []byte
	data          []byte
	all           []byte
}

func NewNrpePacket() *nrpePacket {
	packet := nrpePacket{
		all: make([]byte, nrpePacketLength),
	}
	packet.packetVersion = packet.all[0:2]
	packet.packetType = packet.all[2:4]
	packet.crc32 = packet.all[4:8]
	packet.statusCode = packet.all[8:10]
	packet.data = packet.all[10 : nrpePacketLength-2]

	return &packet
}

// Read reads packet from the wire.
func (p *nrpePacket) Read(conn net.Conn) error {
	n, err := conn.Read(p.all)
	if err != nil {
		return fmt.Errorf("reading request failed: %s", err.Error())
	}

	if n != len(p.all) {
		return fmt.Errorf("nrpe: error while reading")
	}

	return nil
}

// Write sends the packet content to given connection.
func (p *nrpePacket) Write(conn net.Conn) error {
	n, err := conn.Write(p.all)
	if err != nil {
		return fmt.Errorf("nrpe write response failed: %s", err.Error())
	}

	if n != len(p.all) {
		return fmt.Errorf("nrpe: incomplete response")
	}

	return nil
}

// BuildCRC32 returns the crc32 checksum.
func (p *nrpePacket) BuildCRC32() uint32 {
	return crc32.Checksum(p.all, crc32.IEEETable)
}

// Verify checks type and the crc32 checksum.
func (p *nrpePacket) Verify(packetType uint16) error {
	rpt := binary.BigEndian.Uint16(p.packetType)
	if rpt != packetType {
		return fmt.Errorf("nrpe: response packet type mismatch %d != %d", rpt, packetType)
	}

	crc := binary.BigEndian.Uint32(p.crc32)

	binary.BigEndian.PutUint32(p.crc32, 0)

	if crc != p.BuildCRC32() {
		return fmt.Errorf("nrpe: response checksum failed")
	}

	return nil
}
