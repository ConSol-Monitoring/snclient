package nrpe

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNRPERequestV2(t *testing.T) {
	pkgBytes := make([]byte, 1036)
	header, _ := hex.DecodeString("00020001823d740973455f4e5250455f434845434b213121322133")
	copy(pkgBytes, header)

	// add random bytes
	pkgBytes[1034] = '='
	pkgBytes[1035] = 'M'

	pkg, err := ReadNrpePacket(bytes.NewReader(pkgBytes))
	require.NoErrorf(t, err, "read ok")

	assert.Equalf(t, uint16(2), pkg.Version(), "parsed package version")

	require.NoErrorf(t, pkg.Verify(NrpeQueryPacket), "verify ok")

	cmd, args := pkg.Data()
	assert.Equalf(t, "_NRPE_CHECK", cmd, "parsed package command")
	assert.Equalf(t, []string{"1", "2", "3"}, args, "parsed package args")
}

func TestNRPERequestV3(t *testing.T) {
	pkgBytes := make([]byte, 1036)
	header, _ := hex.DecodeString("00030001135b1c7900000000000003f9636865636b5f696e6465782131213221332134")
	copy(pkgBytes, header)

	pkg, err := ReadNrpePacket(bytes.NewReader(pkgBytes))
	require.NoErrorf(t, err, "read ok")

	assert.Equalf(t, uint16(3), pkg.Version(), "parsed package version")

	require.NoErrorf(t, pkg.Verify(NrpeQueryPacket), "verify ok")

	cmd, args := pkg.Data()
	assert.Equalf(t, "check_index", cmd, "parsed package command")
	assert.Equalf(t, []string{"1", "2", "3", "4"}, args, "parsed package args")
}

func TestNRPERequestV4(t *testing.T) {
	pkgBytes := make([]byte, 1036)
	header, _ := hex.DecodeString("0004000158695ec800000000000003fc636865636b5f696e6465782131213221332134")
	copy(pkgBytes, header)

	pkg, err := ReadNrpePacket(bytes.NewReader(pkgBytes))
	require.NoErrorf(t, err, "read ok")

	assert.Equalf(t, uint16(4), pkg.Version(), "parsed package version")

	require.NoErrorf(t, pkg.Verify(NrpeQueryPacket), "verify ok")

	cmd, args := pkg.Data()
	assert.Equalf(t, "check_index", cmd, "parsed package command")
	assert.Equalf(t, []string{"1", "2", "3", "4"}, args, "parsed package args")
}

func TestNRPEResponseV2(t *testing.T) {
	pkgBytes, _ := os.ReadFile("data/v2.0")
	pkg, err := ReadNrpePacket(bytes.NewReader(pkgBytes))
	require.NoErrorf(t, err, "read ok")

	assert.Equalf(t, uint16(2), pkg.Version(), "parsed package version")

	require.NoErrorf(t, pkg.Verify(NrpeResponsePacket), "verify ok")

	data, args := pkg.Data()
	assert.Equalf(t, "USERS WARNING - 8 users currently logged in |users=8;5;10;0", data, "parsed package data")
	assert.Nil(t, args, "no args in response package")
}

func TestNRPEResponseV2_II(t *testing.T) {
	pkgBytes, _ := os.ReadFile("data/v2.1")
	pkg, err := ReadNrpePacket(bytes.NewReader(pkgBytes))
	require.NoErrorf(t, err, "read ok")

	assert.Equalf(t, uint16(2), pkg.Version(), "parsed package version")

	require.NoErrorf(t, pkg.Verify(NrpeResponsePacket), "verify ok")

	exp := "OK - here is the testfile content:\n"
	for i := 1; i <= 7; i++ {
		exp += "test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123\n"
	}
	exp += "test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 "
	data, args := pkg.Data()
	assert.Equalf(t, exp, data, "parsed package data")
	assert.Nil(t, args, "no args in response package")
}

func TestNRPEResponseV4(t *testing.T) {
	pkgBytes, _ := os.ReadFile("data/v4")
	pkg, err := ReadNrpePacket(bytes.NewReader(pkgBytes))
	require.NoErrorf(t, err, "read ok")

	assert.Equalf(t, uint16(4), pkg.Version(), "parsed package version")

	require.NoErrorf(t, pkg.Verify(NrpeResponsePacket), "verify ok")

	exp := "OK - here is the testfile content:\n"
	for i := 1; i <= 47; i++ {
		exp += "test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123\n"
	}
	exp += "test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123 test test test 123 123 123 123"
	data, args := pkg.Data()
	assert.Equalf(t, exp, data, "parsed package data")
	assert.Nil(t, args, "no args in response package")
}
