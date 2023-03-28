package nrpe

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNRPEV2(t *testing.T) {
	t.Parallel()

	nrpeV2Bytes := make([]byte, 0, 1036)
	header, _ := hex.DecodeString("00020001823d740973455f4e5250455f434845434b213121322133")
	nrpeV2Bytes = append(nrpeV2Bytes, header...)

	null := make([]byte, 1036-len(header))
	nrpeV2Bytes = append(nrpeV2Bytes, null...)

	// add checksum
	nrpeV2Bytes = append(nrpeV2Bytes, []byte{'=', 'M'}...)
	pkg := NewNrpePacket()
	pkg.Read(bytes.NewReader(nrpeV2Bytes))

	assert.Equalf(t, uint16(2), pkg.Version(), "parsed package version")

	cmd, args := pkg.Data()
	assert.Equalf(t, "_NRPE_CHECK", cmd, "parsed package command")
	assert.Equalf(t, []string{"1", "2", "3"}, args, "parsed package args")
}
