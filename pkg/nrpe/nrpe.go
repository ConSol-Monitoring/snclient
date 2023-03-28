package nrpe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
)

const (
	nrpeMaxPacketDataLength = 1024
	nrpePacketLength        = nrpeMaxPacketDataLength + 12
	nrpePacketVersion2      = 2
)

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

// BuildPacketV2 creates packet structure.
func BuildPacketV2(packetType, statusCode uint16, statusLine []byte) *NrpePacket {
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

// Version returns nrpe pkg version.
func (p *NrpePacket) Version() uint16 {
	return binary.BigEndian.Uint16(p.packetVersion)
}

// Data returns nrpe payload.
func (p *NrpePacket) Data() (cmd string, args []string) {
	pos := bytes.IndexByte(p.data, 0)

	data := strings.Split(string(p.data[:pos]), "!")

	return data[0], data[1:]
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

	// first nul byte separates command from args
	pos := bytes.IndexByte(p.data, 0)
	if pos == -1 {
		return fmt.Errorf("nrpe: invalid request")
	}

	return nil
}
