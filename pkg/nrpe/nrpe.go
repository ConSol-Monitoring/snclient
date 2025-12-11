package nrpe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

/*
 * nrpe protocol v2/v3 is explained here
 * https://github.com/stockholmuniversity/Nagios-NRPE/blob/master/share/protocol-nrpe.md
 *
 * there is a perl implementation for v2/v3 here:
 * https://metacpan.org/dist/Nagios-NRPE/source/lib/Nagios/NRPE/Packet.pm
 *
 * reference implementation is here:
 * https://github.com/NagiosEnterprises/nrpe/blob/master/src/nrpe.c
 *
 * there is a v2 implementation in golang here:
 * https://github.com/envimate/nrpe/blob/master/nrpe.go
 */

const (
	NrpeV2MaxPacketDataLength = 1024
	NrpeV2PacketLength        = NrpeV2MaxPacketDataLength + 12
	NrpeV2PacketVersion       = 2

	NrpeV3PacketVersion = 3

	NrpeV4PacketVersion       = 4
	NrpeV4HeaderLength        = 16
	NrpeV4MaxPacketDataLength = 65536

	// id code for a packet containing a query.
	NrpeQueryPacket = 1
	// id code for a packet containing a response.
	NrpeResponsePacket = 2
)

// Packet stores nrpe request / response packet.
type Packet struct {
	packetVersion []byte
	packetType    []byte
	crc32         []byte
	statusCode    []byte
	alignment     []byte
	dataLength    []byte
	data          []byte
	all           []byte
}

func NewNrpePacket() *Packet {
	packet := Packet{
		all: make([]byte, NrpeV2PacketLength),
	}

	return &packet
}

// BuildPacket creates packet structure.
func BuildPacket(version, packetType, statusCode uint16, statusLine []byte) *Packet {
	switch version {
	case NrpeV2PacketVersion, NrpeV3PacketVersion:
		return BuildPacketV2(packetType, statusCode, statusLine)
	case NrpeV4PacketVersion:
		return BuildPacketV4(packetType, statusCode, statusLine)
	default:
		return nil
	}
}

// BuildPacketV2 creates new v2 packet structure.
func BuildPacketV2(packetType, statusCode uint16, statusLine []byte) *Packet {
	packet := NewNrpePacket()

	// let slices point to the right locations
	// nrpe v2 uses a fixed sized package with 4 headers followed by 1024 bytes of data
	packet.packetVersion = packet.all[0:2]
	packet.packetType = packet.all[2:4]
	packet.crc32 = packet.all[4:8]
	packet.statusCode = packet.all[8:10]
	packet.data = packet.all[10 : NrpeV2PacketLength-2]

	binary.BigEndian.PutUint16(packet.packetVersion, NrpeV2PacketVersion)
	binary.BigEndian.PutUint16(packet.packetType, packetType)
	binary.BigEndian.PutUint32(packet.crc32, 0)
	binary.BigEndian.PutUint16(packet.statusCode, statusCode)

	length := len(statusLine)

	if length >= NrpeV2MaxPacketDataLength {
		length = NrpeV2MaxPacketDataLength - 1
	}

	copy(packet.data, statusLine[:length])
	packet.data[length] = 0 // add null byte

	binary.BigEndian.PutUint32(packet.crc32, packet.BuildCRC32())

	return packet
}

// BuildPacketV4 creates new v4 packet structure.
func BuildPacketV4(packetType, statusCode uint16, statusLine []byte) *Packet {
	dataLength := len(statusLine) + 1 // +1 for the final null byte
	if dataLength > NrpeV4MaxPacketDataLength {
		statusLine = statusLine[0:NrpeV4MaxPacketDataLength]
		dataLength = len(statusLine)
	}
	if dataLength < NrpeV2MaxPacketDataLength {
		dataLength = NrpeV2MaxPacketDataLength
	}
	packet := &Packet{
		all: make([]byte, NrpeV4HeaderLength+dataLength),
	}

	// let slices point to the right locations
	// nrpe v4 uses a dynamic sized package with 6 fixed headers
	// followed by dynamic sized bytes of data
	packet.packetVersion = packet.all[0:2]
	packet.packetType = packet.all[2:4]
	packet.crc32 = packet.all[4:8]
	packet.statusCode = packet.all[8:10]
	packet.alignment = packet.all[10:12]
	packet.dataLength = packet.all[12:16]
	packet.data = packet.all[NrpeV4HeaderLength : dataLength+NrpeV4HeaderLength]

	binary.BigEndian.PutUint16(packet.packetVersion, NrpeV4PacketVersion)
	binary.BigEndian.PutUint16(packet.packetType, packetType)
	binary.BigEndian.PutUint32(packet.crc32, 0)
	binary.BigEndian.PutUint16(packet.statusCode, statusCode)
	binary.BigEndian.PutUint16(packet.alignment, 0)
	dataLength32, err := convert.UInt32E(dataLength)
	if err != nil {
		panic("should not happen, size is checked earlier already")
	}
	binary.BigEndian.PutUint32(packet.dataLength, dataLength32)

	copy(packet.data, statusLine)
	packet.data[dataLength-1] = 0 // add null byte

	binary.BigEndian.PutUint32(packet.crc32, packet.BuildCRC32())

	return packet
}

// Version returns nrpe pkg version.
func (p *Packet) Version() uint16 {
	return binary.BigEndian.Uint16(p.packetVersion)
}

// Data returns nrpe payload.
func (p *Packet) Data() (cmd string, args []string) {
	rpt := binary.BigEndian.Uint16(p.packetType)
	pos := bytes.IndexByte(p.data, 0)

	if rpt == NrpeResponsePacket {
		return string(p.data[:pos]), nil
	}

	data := strings.Split(string(p.data[:pos]), "!")

	return data[0], data[1:]
}

// Read reads packet from the wire.
func ReadNrpePacket(conn io.Reader) (*Packet, error) {
	packet := NewNrpePacket()

	// read first 1036 bytes, all packages have at least this size
	n, err := conn.Read(packet.all)
	if err != nil {
		return nil, fmt.Errorf("reading request failed: %s", err.Error())
	}
	if n != len(packet.all) {
		return nil, fmt.Errorf("nrpe: error while reading")
	}

	packet.parseSize()

	switch packet.Version() {
	case NrpeV2PacketVersion:
		packet.data = packet.all[10 : NrpeV2PacketLength-2]
	case NrpeV3PacketVersion, NrpeV4PacketVersion:
		dataLength := binary.BigEndian.Uint32(packet.dataLength)
		// 1020 bytes is the amount which comes with reading the first 1036 bytes - header (16 bytes)
		remaining := int64(dataLength) - (NrpeV2PacketLength - NrpeV4HeaderLength)
		if remaining <= 0 {
			packet.data = packet.all[16:NrpeV2PacketLength]
		} else {
			// larger than 1020 bytes, read the remaining data
			body := new(bytes.Buffer)
			n, err := io.CopyN(body, conn, remaining)
			if err != nil {
				return nil, fmt.Errorf("nrpe error, failed to read remaining data: %s", err.Error())
			}
			if n != remaining {
				return nil, fmt.Errorf("nrpe error, failed to read remaining data, read %d instead of %d", n, remaining)
			}
			packet.all = append(packet.all, body.Bytes()...)
			packet.parseSize() // update header locations
			packet.data = packet.all[16 : dataLength+16]
		}
	}

	return packet, nil
}

func (p *Packet) parseSize() {
	// version is always first 2 bytes
	p.packetVersion = p.all[0:2]

	// package type is always second 2 bytes
	p.packetType = p.all[2:4]

	// crc32 is always the 3rd header
	p.crc32 = p.all[4:8]

	switch p.Version() {
	case NrpeV2PacketVersion:
		p.statusCode = p.all[8:10]
	case NrpeV3PacketVersion, NrpeV4PacketVersion:
		p.statusCode = p.all[8:10]
		p.alignment = p.all[10:12]
		p.dataLength = p.all[12:16]
	}
}

// Write sends the packet content to given connection.
func (p *Packet) Write(conn io.Writer) error {
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
func (p *Packet) BuildCRC32() uint32 {
	// checksum is build over the hole package but with the crc bytes nulled
	savedCheckSum := binary.BigEndian.Uint32(p.crc32)
	binary.BigEndian.PutUint32(p.crc32, 0)

	checkSum := crc32.Checksum(p.all, crc32.IEEETable)

	// restore original value
	binary.BigEndian.PutUint32(p.crc32, savedCheckSum)

	return checkSum
}

// Verify checks type and the crc32 checksum.
func (p *Packet) Verify(packetType uint16) error {
	rpt := binary.BigEndian.Uint16(p.packetType)
	if rpt != packetType {
		return fmt.Errorf("nrpe: response packet type mismatch %d != %d", rpt, packetType)
	}

	crc := binary.BigEndian.Uint32(p.crc32)
	if crc != p.BuildCRC32() {
		return fmt.Errorf("nrpe: response checksum failed: %v != %v", crc, p.BuildCRC32())
	}

	// first nul byte separates command from args
	if rpt == NrpeQueryPacket {
		pos := bytes.IndexByte(p.data, 0)
		if pos == -1 {
			return fmt.Errorf("nrpe: invalid request")
		}
	}

	return nil
}
