package snclient

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus"
)

// nrpePacket stores nrpe request / response packet.
type nrpePacket struct {
	packetVersion []byte
	packetType    []byte
	crc32         []byte
	statusCode    []byte
	data          []byte
	all           []byte
}

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
		"socket_timeout":         "30",
	}

	return defaults
}

var randSource *rand.Rand

func (l *HandlerNRPE) Init(snc *SNClientInstance) error {
	var crc, poly, i, j uint32

	crc32Table = make([]uint32, 256)

	poly = uint32(0xEDB88320)

	for i = 0; i < 256; i++ {
		crc = i

		for j = 8; j > 0; j-- {
			if (crc & 1) != 0 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}

		crc32Table[i] = crc
	}

	randSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	return nil
}

func (l *HandlerNRPE) ServeOne(snc *SNClientInstance, con net.Conn) {
	defer con.Close()

	request := l.createPacket()

	if err := l.readPacket(con, request); err != nil {
		log.Errorf("nrpe protocol error: %s", err.Error())

		return
	}

	if err := l.verifyPacket(request, nrpeQueryPacketType); err != nil {
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

	if err := l.writePacket(con, response); err != nil {
		log.Errorf("nrpe write response error: %s", err.Error())

		return
	}
}

func (l *HandlerNRPE) createPacket() *nrpePacket {
	var p nrpePacket
	p.all = make([]byte, nrpePacketLength)

	p.packetVersion = p.all[0:2]
	p.packetType = p.all[2:4]
	p.crc32 = p.all[4:8]
	p.statusCode = p.all[8:10]
	p.data = p.all[10 : nrpePacketLength-2]

	return &p
}

// readPacket reads from connection to packet.
func (l *HandlerNRPE) readPacket(conn net.Conn, p *nrpePacket) error {
	// TODO: move to listener
	/*
		if timeout > 0 {
			conn.SetReadDeadline(time.Now().Add(timeout))
		}
	*/

	n, err := conn.Read(p.all)
	if err != nil {
		return err
	}

	if n != len(p.all) {
		return fmt.Errorf("nrpe: error while reading")
	}

	return nil
}

// verifyPacket checks packetType and crc32.
func (l *HandlerNRPE) verifyPacket(responsePacket *nrpePacket, packetType uint16) error {
	be := binary.BigEndian

	rpt := be.Uint16(responsePacket.packetType)
	if rpt != packetType {
		return fmt.Errorf(
			"nrpe: Error response packet type, got: %d, expected: %d",
			rpt, packetType)
	}

	crc := be.Uint32(responsePacket.crc32)

	be.PutUint32(responsePacket.crc32, 0)

	if crc != l.crc32(responsePacket.all) {
		return fmt.Errorf("nrpe: Response crc didn't match")
	}

	return nil
}

var crc32Table []uint32

// Builds crc32 from the given input
func (l *HandlerNRPE) crc32(in []byte) uint32 {
	var crc uint32

	crc = uint32(0xFFFFFFFF)

	for _, c := range in {
		crc = ((crc >> 8) & uint32(0x00FFFFFF)) ^ crc32Table[(crc^uint32(c))&0xFF]
	}

	return (crc ^ uint32(0xFFFFFFFF))
}

// buildPacket creates packet structure.
func (l *HandlerNRPE) buildPacket(packetType uint16, statusCode uint16, statusLine []byte) *nrpePacket {
	be := binary.BigEndian
	p := l.createPacket()

	l.randomizeBuffer(p.all)

	be.PutUint16(p.packetVersion, nrpePacketVersion2)
	be.PutUint16(p.packetType, packetType)
	be.PutUint32(p.crc32, 0)
	be.PutUint16(p.statusCode, statusCode)

	length := len(statusLine)

	if length >= nrpeMaxPacketDataLength {
		length = nrpeMaxPacketDataLength - 1
	}

	copy(p.data, statusLine[:length])
	p.data[length] = 0

	be.PutUint32(p.crc32, l.crc32(p.all))

	return p
}

// writePacket writes packet content to connection
func (l *HandlerNRPE) writePacket(conn net.Conn, p *nrpePacket) error {
	/* TODO: ...
	if timeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	*/

	n, err := conn.Write(p.all)
	if err != nil {
		return err
	}

	if n != len(p.all) {
		return fmt.Errorf("nrpe: error while writing")
	}

	return nil
}

// extra randomization for encryption.
func (l *HandlerNRPE) randomizeBuffer(in []byte) {
	n := len(in) >> 2

	for i := 0; i < n; i++ {
		r := randSource.Uint32()

		copy(in[i<<2:(i+1)<<2], (*[4]byte)(unsafe.Pointer(&r))[:])
	}

	if len(in)%4 != 0 {
		r := randSource.Uint32()

		copy(in[n<<2:], (*[4]byte)(unsafe.Pointer(&r))[:len(in)-(n<<2)])
	}
}
