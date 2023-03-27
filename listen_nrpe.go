package snclient

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"strings"
)

const (
	nrpeMaxPacketDataLength = 1024
	nrpePacketLength        = nrpeMaxPacketDataLength + 12
	nrpeQueryPacketType     = 1
	nrpeResponsePacketType  = 2
	// currently supporting latest version2 protocol.
	nrpePacketVersion2 = 2
)

func init() {
	AvailableListeners = append(AvailableListeners, ListenHandler{"NRPEServer", "/settings/NRPE/server", NewHandlerNRPE()})
}

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

func (l *HandlerNRPE) Defaults() ConfigSection {
	defaults := ConfigSection{
		"allow arguments":   "0",
		"allow nasty chars": "0",
		"port":              "5666",
	}
	defaults.Merge(DefaultListenTCPConfig)

	return defaults
}

func (l *HandlerNRPE) Init(_ *Agent) error {
	return nil
}

func (l *HandlerNRPE) ServeTCP(snc *Agent, con net.Conn) {
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

	statusResult := snc.RunCheck(data[0], data[1:])

	output := []byte(statusResult.Output)
	if len(statusResult.Metrics) > 0 {
		output = append(output, '|')

		for _, m := range statusResult.Metrics {
			output = append(output, []byte(m.BuildNaemonString())...)
		}
	}

	response := l.buildPacket(nrpeResponsePacketType, uint16(statusResult.State), output)

	if err := response.Write(con); err != nil {
		log.Errorf("nrpe write response error: %s", err.Error())

		return
	}
}

// buildPacket creates packet structure.
func (l *HandlerNRPE) buildPacket(packetType, statusCode uint16, statusLine []byte) *NrpePacket {
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

// NrpePacket stores nrpe request / response packet.
type NrpePacket struct {
	packetVersion []byte
	packetType    []byte
	crc32         []byte
	statusCode    []byte
	data          []byte
	all           []byte
}

func NewNrpePacket() *NrpePacket {
	packet := NrpePacket{
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
func (p *NrpePacket) Read(conn io.Reader) error {
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
func (p *NrpePacket) Write(conn io.Writer) error {
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
func (p *NrpePacket) BuildCRC32() uint32 {
	return crc32.Checksum(p.all, crc32.IEEETable)
}

// Verify checks type and the crc32 checksum.
func (p *NrpePacket) Verify(packetType uint16) error {
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